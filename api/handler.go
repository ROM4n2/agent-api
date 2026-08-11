// Package api 提供任务提交与查询的 HTTP 接口。
//
// 这一层只做「解析请求 → 落库 → 入队 → 立刻应答」，不执行任何任务逻辑；
// 真正的执行在 worker 包中异步进行。
package api

import (
	"agent-api/store"
	"agent-api/worker"
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

// Handler 持有处理请求所需的依赖，由 main 装配后注入。
type Handler struct {
	store *store.Store
	pool  *worker.Pool
}

func NewHandler(s *store.Store, p *worker.Pool) *Handler {
	return &Handler{store: s, pool: p}
}

// Routes 返回已注册全部路由的 mux。
//
// 用的是 Go 1.22+ ServeMux 的方法路由语法："METHOD /path"，
// 方法与路径一起参与匹配，因此不必在 handler 内部自己判断 r.Method。
func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /run", h.HandleRun)
	// 路径是复数 tasks，与 HandleGet 的取值 r.PathValue("id") 配套
	mux.HandleFunc("GET /tasks/{id}", h.HandleGet)
	return mux
}

// HandleRun 接收任务并立即返回 task_id，不等待执行结果。
//
// 返回 202 而非 200：任务此刻尚未开始执行，200 的语义是「已处理完成」，
// 用它会误导调用方。调用方应拿 task_id 轮询 GET /tasks/{id}。
func (h *Handler) HandleRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prompt string
	}
	// 限制请求体大小，避免客户端发大文件导致内存耗尽。
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if req.Prompt == "" {
		http.Error(w, "prompt is empty", http.StatusBadRequest)
		return
	}

	// 先落库再入队：Enqueue 之后 worker 可能立刻开始处理，
	// 此时该 ID 必须已经存在于 store 中。
	id := h.store.Create(req.Prompt)
	h.pool.Enqueue(id)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusAccepted)
	err = json.NewEncoder(w).Encode(map[string]string{"task_id": id})

	// 响应头已写出，此时无法再改状态码，编码失败只能记日志。
	// 且这类失败基本只源于客户端断连，重试没有意义。
	if err != nil {
		log.Printf("encode response: %v", err)
	}

}

// HandleGet 返回任务当前状态，供调用方轮询。
func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	// {id} 由 mux 从路径中解析，不是查询参数
	id := r.PathValue("id")

	task, err := h.store.Get(id)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(task)
	if err != nil {
		log.Printf("encode response: %v", err)
	}
}
