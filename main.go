package main

import (
	"agent-api/api"
	"agent-api/config"
	"agent-api/llm"
	"agent-api/store"
	"agent-api/worker"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	os.Exit(run())
}

func run() int {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// 配置来源：环境变量优先，其次 config.yaml（文件可选）。
	conf, err := config.Load("")
	if err != nil {
		slog.Error("load config", slog.Any("error", err))
		return 1
	}
	apiKey := conf.DeepSeekAPIKey
	if apiKey == "" {
		slog.Error("DEEPSEEK_API_KEY is not set（环境变量或 config.yaml 任选其一）")
		return 1
	}

	cfg := llm.Config{
		APIKey:  apiKey,
		BaseURL: "https://api.deepseek.com",
		Model:   "deepseek-chat",
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s := store.NewStore()
	p := worker.NewPool(3, s, llm.NewClient(cfg))

	p.Start()
	defer p.Stop()

	h := api.NewHandler(s, p, conf.APIAuthKey)
	mux := h.Routes()

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	errCh := make(chan error, 1) // 缓冲 1，goroutine 送完就走，不会泄露

	go func() {
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		// 收到信号，正常停机
		return shutdown(srv)

	case err := <-errCh:
		// 启动失败，也要走停机流程，但退出码要非 0
		slog.Error("server error", slog.Any("error", err))
		return 1
	}
}

func shutdown(srv *http.Server) int {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	slog.Info("shutting down server")

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", slog.Any("error", err))
		return 1
	}
	return 0
}
