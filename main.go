package main

import (
	"agent-api/api"
	"agent-api/store"
	"agent-api/worker"
	"log"
	"net/http"
)

func main() {
	s := store.NewStore()
	p := worker.NewPool(3, s)
	p.Start()
	defer p.Stop()

	h := api.NewHandler(s, p)
	mux := h.Routes()
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal(err)
	}
}
