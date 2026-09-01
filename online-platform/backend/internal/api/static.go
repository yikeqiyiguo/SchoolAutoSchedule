package api

import (
	"embed"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed web
var webFS embed.FS

// ExternalWebDir 外部前端静态目录（线上部署时指向 frontend/dist；为空则使用内嵌前端）
var ExternalWebDir string

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
	// 线上模式：优先从外部静态目录读取（如 Vue 构建产物 frontend/dist）
	if ExternalWebDir != "" && !strings.Contains(path, "..") {
		if b, err := os.ReadFile(filepath.Join(ExternalWebDir, filepath.FromSlash(path))); err == nil {
			serveBytes(w, path, b)
			return
		}
		if b, err := os.ReadFile(filepath.Join(ExternalWebDir, "index.html")); err == nil {
			serveBytes(w, "index.html", b)
			return
		}
		http.NotFound(w, r)
		return
	}
	content, err := webFS.ReadFile("web/" + path)
	if err != nil {
		content, err = webFS.ReadFile("web/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	serveBytes(w, path, content)
}

func serveBytes(w http.ResponseWriter, path string, content []byte) {
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
	// 禁止浏览器缓存，避免更新后仍显示旧版前端
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Write(content)
}
