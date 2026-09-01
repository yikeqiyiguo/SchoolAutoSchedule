package api

import (
	"embed"
	"net/http"
	"strings"
)

//go:embed web
var webFS embed.FS

// handleStatic 前端静态资源（SPA 回退 index.html）
func handleStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		fail(w, 404, "接口不存在")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	content, err := webFS.ReadFile("web/" + path)
	if err != nil {
		content, err = webFS.ReadFile("web/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	ct := "text/plain; charset=utf-8"
	switch {
	case strings.HasSuffix(path, ".html"):
		ct = "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		ct = "application/javascript; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		ct = "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".svg"):
		ct = "image/svg+xml"
	case strings.HasSuffix(path, ".png"):
		ct = "image/png"
	case strings.HasSuffix(path, ".ico"):
		ct = "image/x-icon"
	case strings.HasSuffix(path, ".woff2"):
		ct = "font/woff2"
	case strings.HasSuffix(path, ".json"):
		ct = "application/json; charset=utf-8"
	}
	w.Header().Set("Content-Type", ct)
	w.Write(content)
}
