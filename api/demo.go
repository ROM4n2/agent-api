package api

import (
	"embed"
	"net/http"
)

//go:embed demo.html
var demoFS embed.FS

// serveDemo 把内嵌的单页 Demo 挂到 GET /，无需任何外部文件或 CDN，
// 离线即可打开。页面提交到 /run 并轮询 /tasks/{id}。
func serveDemo(w http.ResponseWriter, r *http.Request) {
	data, err := demoFS.ReadFile("demo.html")
	if err != nil {
		http.Error(w, "demo not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}
