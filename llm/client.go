package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const chatCompletionsPath = "/v1/chat/completions"

// 请求
type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// 响应
type chatResponse struct {
	Choices []choice `json:"choices"`
}

type choice struct {
	Message message `json:"message"`
}

// Client 调用 OpenAI 兼容的 chat completions 接口。
type Client struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client // 复用连接，且用来设超时
}

type Config struct {
	APIKey  string
	BaseURL string
	Model   string
}

func NewClient(cfg Config) *Client {
	return &Client{
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// Chat 发送 prompt，返回模型回复的文本。
func (c *Client) Chat(ctx context.Context, prompt string) (string, error) {
	// 1. 构造 chatRequest（model + 一条 user message）
	chatReq := chatRequest{
		Model: c.model,
		Messages: []message{
			{Role: "user", Content: prompt},
		},
	}

	// 2. json.Marshal 成 []byte
	chatReqJSON, err := json.Marshal(chatReq)
	if err != nil {
		return "", fmt.Errorf("llm: marshal request: %w", err)
	}

	// 3. http.NewRequestWithContext 建请求（POST、URL、body）
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+chatCompletionsPath, nil)
	if err != nil {
		return "", fmt.Errorf("llm: create request: %w", err)
	}

	// 4. 设两个 header：Authorization、Content-Type
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+chatCompletionsPath, bytes.NewReader(chatReqJSON))

	// 5. c.http.Do(req) 发送；检查 StatusCode
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("llm: send request: %w", err)
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llm: unexpected status code: %d, error: %s", resp.StatusCode, string(bodyBytes))
	}

	// 6. json.Decode 响应 → 取 choices[0].message.content 返回
	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("llm: decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", errors.New("llm: no choices in response")
	}
	return chatResp.Choices[0].Message.Content, nil
}
