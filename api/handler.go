package api

import (
	"agent-api/store"
	"agent-api/worker"
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

type Handler struct {
	store *store.Store
	pool  *worker.Pool
}

func NewHandler(s *store.Store, p *worker.Pool) *Handler {
	return &Handler{store: s, pool: p}
}

func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /run", h.HandleRun)
	// 注意: 注册的是 tasks
	mux.HandleFunc("GET /tasks/{id}", h.HandleGet)
	return mux
}

func (h *Handler) HandleRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prompt string
	}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if req.Prompt == "" {
		http.Error(w, "prompt is empty", http.StatusBadRequest)
		return
	}

	id := h.store.Create(req.Prompt)
	h.pool.Enqueue(id)

	w.WriteHeader(http.StatusAccepted)
	err = json.NewEncoder(w).Encode(map[string]string{"task_id": id})

	if err != nil {
		log.Printf("encode response: %v", err)
	}

}

func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	task, err := h.store.Get(id)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(task)
	if err != nil {
		log.Printf("encode response: %v", err)
	}
}
