// 学校智能自定义排课系统 - 线上服务版主程序
// 纯 HTTP 服务（无托盘/无浏览器打开），适合部署到服务器，前端由 --web-dir 指向的 Vue 构建产物提供
package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"school-scheduler/internal/api"
	"school-scheduler/internal/backup"
	"school-scheduler/internal/config"
	"school-scheduler/internal/store"
)

func main() {
	host := flag.String("host", "", "监听地址（默认 0.0.0.0）")
	port := flag.String("port", "", "监听端口（默认 8000）")
	dataDir := flag.String("data-dir", "", "数据目录（默认 exe 同级 data/）")
	webDir := flag.String("web-dir", "", "前端静态目录（Vue 构建产物 dist，默认内置前端）")
	flag.Parse()

	if *host != "" {
		os.Setenv("SAS_HOST", *host)
	}
	if *port != "" {
		os.Setenv("SAS_PORT", *port)
	}
	if *dataDir != "" {
		os.Setenv("SAS_DATA_DIR", *dataDir)
	}
	if *webDir != "" {
		api.ExternalWebDir = *webDir
	}

	if err := config.Init(); err != nil {
		log.Fatalf("初始化失败: %v", err)
	}
	// 线上默认监听 0.0.0.0，便于服务器外网访问
	if os.Getenv("SAS_HOST") == "" {
		config.Host = "0.0.0.0"
	}
	if err := store.Open(); err != nil {
		log.Fatalf("数据库打开失败: %v", err)
	}
	if api.ExternalWebDir != "" {
		if st, err := os.Stat(api.ExternalWebDir); err != nil || !st.IsDir() {
			log.Printf("警告: --web-dir 目录不存在，将使用内置前端: %s", api.ExternalWebDir)
			api.ExternalWebDir = ""
		}
	}

	// 每日自动备份
	go autoBackupCheck()

	addr := config.Host + ":" + config.Port
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("端口 %s 被占用: %v", config.Port, err)
	}
	log.Printf("线上服务已启动: http://%s  (数据目录: %s, 前端: %s)",
		displayURL(), config.DataDir, frontDesc())
	if err := http.Serve(ln, api.NewServer().Handler()); err != nil {
		log.Fatalf("HTTP 服务异常: %v", err)
	}
}

func displayURL() string {
	h := config.Host
	if h == "0.0.0.0" || h == "" {
		h = "localhost"
	}
	return h + ":" + config.Port
}

func frontDesc() string {
	if api.ExternalWebDir != "" {
		return "外部目录 " + api.ExternalWebDir
	}
	return "内置前端"
}

// autoBackupCheck 每日自动备份一次
func autoBackupCheck() {
	marker := filepath.Join(config.DataDir, "last_auto_backup.txt")
	today := time.Now().Format("2006-01-02")
	last := ""
	if b, err := os.ReadFile(marker); err == nil {
		last = string(b)
	}
	if last != today {
		if _, err := backup.Create("auto"); err != nil {
			log.Printf("自动备份失败: %v", err)
		} else {
			os.WriteFile(marker, []byte(today), 0o644)
			log.Printf("已完成每日自动备份")
		}
	}
}
