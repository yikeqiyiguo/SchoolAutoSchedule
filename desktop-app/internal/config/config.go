// Package config 提供程序路径与运行参数配置
package config

import (
	"os"
	"path/filepath"
)

var (
	ExeDir    string // exe 所在目录
	DataDir   string // 数据库目录
	BackupDir string // 备份目录
	LogDir    string // 日志目录
	DBPath    string // 数据库文件
	Host      string
	Port      string
)

// Init 初始化运行目录与参数
func Init() error {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	ExeDir, err = filepath.Abs(filepath.Dir(exe))
	if err != nil {
		ExeDir = filepath.Dir(exe)
	}
	base := os.Getenv("SAS_DATA_DIR")
	if base == "" {
		base = ExeDir
	}
	DataDir = filepath.Join(base, "data")
	BackupDir = filepath.Join(base, "backups")
	LogDir = filepath.Join(base, "logs")
	for _, d := range []string{DataDir, BackupDir, LogDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	DBPath = filepath.Join(DataDir, "scheduler.db")
	Host = getEnv("SAS_HOST", "127.0.0.1")
	Port = getEnv("SAS_PORT", "8899")
	return nil
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// URL 返回本机访问地址
func URL() string {
	return "http://" + Host + ":" + Port
}
