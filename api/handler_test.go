package api

import (
	"agent-api/store"
	"agent-api/worker"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeLLM struct{}

func (fakeLLM) Chat(ctx context.Context, prompt string) (string, error) {
	return "fake response", nil
}

func newTestMux() (*http.ServeMux, *store.Store) {
	s := store.NewStore()
	p := worker.NewPool(3, s, fakeLLM{})
	h := NewHandler(s, p)
	return h.Routes(), s
}

func TestHandleRun_CreatesTask(t *testing.T) {
	mux, s := newTestMux()

	req, err := http.NewRequest("POST", "/run", strings.NewReader(`{"prompt":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	// 断言 1：状态码
	if rec.Code != http.StatusAccepted {
		t.Errorf("code = %d, want 202", rec.Code)
	}

	// 断言 2：解析 body 拿 ta
	var resp map[string]string
	err = json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatal(err)
	}
	if resp["task_id"] == "" {
		t.Fatalf("task_id 为空")
	}

	// 断言 3：store 里有这任务，pending + prompt
	task, err := s.Get(resp["task_id"])
	if err != nil {
		t.Fatalf("Get 报错 %v", err)
	}
	if task.Prompt != "hi" {
		t.Errorf("prompt = %q, want hi", task.Prompt)
	}
	if task.Status != store.StatusPending {
		t.Errorf("status = %q, want pending", task.Status)
	}
}

func TestHandleRun_BadJSON(t *testing.T) {
	mux, _ := newTestMux()

	req, err := http.NewRequest("POST", "/run", strings.NewReader(`{"prompt"`))
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", rec.Code)
	}

}

func TestHandleRun_EmptyPrompt(t *testing.T) {
	mux, _ := newTestMux()
	req, err := http.NewRequest("POST", "/run", strings.NewReader(`{"prompt":""}`))
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", rec.Code)
	}
}

func TestHandleGet_Found(t *testing.T) {
	mux, _ := newTestMux()

	// POST 建任务，拿真 task_id
	postReq := httptest.NewRequest("POST", "/run", strings.NewReader(`{"prompt":"hi"}`))
	postRec := httptest.NewRecorder()
	mux.ServeHTTP(postRec, postReq)

	var resp map[string]string
	err := json.NewDecoder(postRec.Body).Decode(&resp)
	if err != nil {
		t.Fatal(err)
	}
	id := resp["task_id"]

	// GET 真单号 -> 应该 200
	getReq := httptest.NewRequest("GET", "/tasks/"+id, nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", getRec.Code)
	}

	// 解析 body 中的 status
	var body struct {
		Status string
	}

	err = json.NewDecoder(getRec.Body).Decode(&body)
	if err != nil {
		t.Fatal(err)
	}

	if body.Status == "" {
		t.Errorf("status 为空")
	}
}

func TestHandleGet_NotFound(t *testing.T) {
	mux, _ := newTestMux()
	req, err := http.NewRequest("GET", "/tasks/"+"123", nil)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", rec.Code)
	}

}
