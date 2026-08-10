package main

import (
	"agent-api/api"
	"agent-api/store"
	"agent-api/worker"
	"net/http"
)

func main() {
	s := store.NewStore()
	p := worker.NewPool(3, s)
	p.Start()
	defer p.Stop()

	h := api.NewHandler(s, p)
	mux := h.Routes()
	http.ListenAndServe(":8080", mux)
}
