// 学校智能自定义排课系统 - 桌面版主程序
// 架构：Go 内置 HTTP 服务 + embed 前端 + 系统托盘 + 自动打开浏览器
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/getlantern/systray"

	"school-scheduler/internal/api"
	"school-scheduler/internal/backup"
	"school-scheduler/internal/config"
	"school-scheduler/internal/store"
)

var httpServer *http.Server

func main() {
	// 1. 初始化目录与参数
	if err := config.Init(); err != nil {
		logf("初始化失败: %v", err)
		return
	}
	logf("程序启动，数据目录: %s", config.DataDir)

	// 2. 打开数据库
	if err := store.Open(); err != nil {
		logf("数据库打开失败: %v", err)
		showError("数据库初始化失败：" + err.Error())
		return
	}

	// 3. 每日自动备份检查
	go autoBackupCheck()

	// 4. 启动 HTTP 服务（单实例：端口被占说明已运行）
	url := config.URL()
	addr := config.Host + ":" + config.Port
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		logf("端口 %s 已被占用（程序可能已在运行），仅打开浏览器", config.Port)
		openBrowser(url)
		return
	}
	httpServer = &http.Server{Handler: api.NewServer().Handler()}
	go func() {
		if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			logf("HTTP 服务异常: %v", err)
		}
	}()
	logf("系统已启动: %s", url)

	// 5. 系统托盘（headless 模式仅运行服务，供自动化测试）
	if isHeadless() {
		logf("headless 模式运行中（Ctrl+C 退出）")
		select {}
	}
	systray.Run(onReady, onExit)
}

func isHeadless() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SAS_HEADLESS"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func onReady() {
	icon := makeIcon()
	systray.SetIcon(icon)
	systray.SetTitle("学校智能排课系统")
	systray.SetTooltip("学校智能排课系统 · 运行中\n双击图标打开系统")

	mOpen := systray.AddMenuItem("打开排课系统", "在浏览器中打开")
	mSite := systray.AddMenuItem("打开数据目录", "打开 data/backups 数据文件夹")
	mQuit := systray.AddMenuItem("退出系统", "退出并停止排课服务")

	url := config.URL()
	openBrowser(url)

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				openBrowser(url)
			case <-mSite.ClickedCh:
				openFolder(config.DataDir)
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {
	if httpServer != nil {
		httpServer.Close()
	}
	logf("程序已退出")
}

/* ---------- 工具 ---------- */

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
			logf("自动备份失败: %v", err)
		} else {
			os.WriteFile(marker, []byte(today), 0o644)
			logf("已完成每日自动备份")
		}
	}
}

// openBrowser 打开默认浏览器
func openBrowser(url string) {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	cmd.Start()
}

// openFolder 打开资源管理器
func openFolder(dir string) {
	os.MkdirAll(dir, 0o755)
	exec.Command("explorer.exe", dir).Start()
}

// makeIcon 运行时生成托盘图标（蓝色日历样式）
func makeIcon() []byte {
	const s = 32
	img := image.NewRGBA(image.Rect(0, 0, s, s))
	for y := 0; y < s; y++ {
		for x := 0; x < s; x++ {
			t := float64(y) / s
			c := color.RGBA{uint8(59 + 40*t), uint8(110 + 20*t), uint8(246 - 40*t), 255}
			// 圆角
			if (x < 2 || y < 2 || x >= s-2 || y >= s-2) && (x < s && y < s) {
				c = color.RGBA{0, 0, 0, 255}
			}
			// 白色"日历格"图案
			if x >= 4 && x < 28 && y >= 14 && y < 30 && (x-4)%6 < 5 && (y-14)%4 < 3 {
				c = color.RGBA{255, 255, 255, 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

var logFile *os.File

func logf(format string, args ...interface{}) {
	msg := fmt.Sprintf("%s [INFO] %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, args...))
	if logFile == nil {
		os.MkdirAll(config.LogDir, 0o755)
		logFile, _ = os.OpenFile(filepath.Join(config.LogDir, "app.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	}
	if logFile != nil {
		logFile.WriteString(msg)
	}
}

// showError 弹窗提示（GUI 模式无控制台时用）
func showError(msg string) {
	exec.Command("cmd", "/c", "echo "+msg+" & pause").Run()
}
