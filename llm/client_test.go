package llm

import (
	"testing"
)

func TestChat_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. 可选但强烈建议：在这里检查请求
		//    r.Method 是不是 POST？r.URL.Path 是不是 /v1/chat/completions？
		//    r.Header.Get("Authorization") 是不是 "Bearer test-key"？
		// 2. 写回一段合法的 chat completions JSON
	}))
	defer srv.Close()

	c := NewClient(Config{APIKey: ..., BaseURL: srv.URL, Model: ...})
	got, err := c.Chat(context.Background(), "hi")
	// 断言：err 为 nil，got 等于你在上面 JSON 里填的 content
}

func TestChat_APIError(t *testing.T) {
	// handler：w.WriteHeader(401) + 写一段错误 body
	// 断言：errors.As 能取出 *APIError；它的 StatusCode == 401
}

func TestChat_Timeout(t *testing.T) {
	// handler：先 time.Sleep 一段（比如 200ms），什么都不写
	// ctx：context.WithTimeout(context.Background(), 50ms)，记得 defer cancel()
	// 断言：errors.Is(err, context.DeadlineExceeded)
}
