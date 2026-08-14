package main

import (
	"agent-api/api"
	"agent-api/llm"
	"agent-api/store"
	"agent-api/worker"
	"log"
	"log/slog"
	"net/http"
	"os"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		log.Fatal("DEEPSEEK_API_KEY is not set")
	}

	cfg := llm.Config{
		APIKey:  apiKey,
		BaseURL: "https://api.deepseek.com",
		Model:   "deepseek-chat",
	}

	s := store.NewStore()
	p := worker.NewPool(3, s, llm.NewClient(cfg))

	p.Start()
	defer p.Stop()

	h := api.NewHandler(s, p)
	mux := h.Routes()
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal(err)
	}
}
