// Package backup 提供数据库备份与恢复
package backup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"school-scheduler/internal/config"
	"school-scheduler/internal/store"
)

// Create 创建备份，返回文件名
func Create(backupType string) (string, error) {
	name := fmt.Sprintf("backup-%s-%s.db", time.Now().Format("20060102-150405"), backupType)
	target := filepath.Join(config.BackupDir, name)
	if _, err := os.Stat(target); err == nil {
		os.Remove(target)
	}
	// VACUUM INTO 生成一致性快照
	stmt := "VACUUM INTO '" + target + "'"
	if _, err := store.DB.Exec(stmt); err != nil {
		return "", fmt.Errorf("备份失败: %w", err)
	}
	fi, err := os.Stat(target)
	if err != nil {
		return "", err
	}
	_, err = store.DB.Exec("INSERT INTO backups (filename, file_size, backup_type) VALUES (?,?,?)",
		name, fi.Size(), backupType)
	return name, err
}

// List 备份列表
func List() []map[string]interface{} {
	rows, err := store.DB.Query("SELECT id, filename, file_size, backup_type, created_at FROM backups ORDER BY id DESC")
	if err != nil {
		return []map[string]interface{}{}
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var filename, backupType, createdAt string
		var size int64
		if err := rows.Scan(&id, &filename, &size, &backupType, &createdAt); err != nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"id": id, "filename": filename, "file_size": size,
			"file_size_text": fmtSize(size), "backup_type": backupType, "created_at": createdAt,
		})
	}
	return out
}

func fmtSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.2f MB", float64(n)/(1024*1024))
}

// GetFile 获取备份文件路径
func GetFile(id int) (string, error) {
	var filename string
	if err := store.DB.QueryRow("SELECT filename FROM backups WHERE id=?", id).Scan(&filename); err != nil {
		return "", errors.New("备份不存在")
	}
	p := filepath.Join(config.BackupDir, filename)
	if _, err := os.Stat(p); err != nil {
		return "", errors.New("备份文件已丢失")
	}
	return p, nil
}

// Delete 删除备份
func Delete(id int) error {
	if p, err := GetFile(id); err == nil {
		os.Remove(p)
	}
	_, err := store.DB.Exec("DELETE FROM backups WHERE id=?", id)
	return err
}

// Restore 用备份恢复数据库（恢复前自动备份当前数据）
func Restore(id int) (string, error) {
	src, err := GetFile(id)
	if err != nil {
		return "", err
	}
	// 1. 先备份当前数据（自动类型）
	prevName, err := Create("pre-restore")
	if err != nil {
		return "", fmt.Errorf("恢复前备份失败: %w", err)
	}
	// 2. 备份当前数据库文件
	backupCur := filepath.Join(config.DataDir, "current-before-restore.db")
	os.Remove(backupCur)
	stmt := "VACUUM INTO '" + backupCur + "'"
	store.DB.Exec(stmt)

	// 3. 关闭连接并覆盖（恢复的是当前学校库）
	curFile := store.CurrentDBFile()
	store.DB.Close()
	if err := copyFile(src, curFile); err != nil {
		store.ReopenCurrentSchool()
		return "", fmt.Errorf("恢复失败: %w", err)
	}
	// 清理 WAL 残留
	os.Remove(curFile + "-wal")
	os.Remove(curFile + "-shm")
	os.Remove(backupCur)
	if err := store.ReopenCurrentSchool(); err != nil {
		return "", fmt.Errorf("恢复后重连失败: %w", err)
	}
	return prevName, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
