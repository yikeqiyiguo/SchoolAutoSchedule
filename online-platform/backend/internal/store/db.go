// Package store 负责 SQLite 数据库连接、建表与种子数据
package store

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"

	"school-scheduler/internal/config"
)

var DB *sql.DB

// Open 打开数据库连接并初始化 schema
func Open() error {
	if err := os.MkdirAll(config.DataDir, 0o755); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", config.DBPath)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1) // SQLite 单写者
	db.SetMaxIdleConns(1)
	DB = db

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=8000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
	}
	if err := initSchema(); err != nil {
		return err
	}
	if err := seedIfEmpty(); err != nil {
		return err
	}
	return nil
}

func initSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			real_name TEXT NOT NULL DEFAULT '',
			role TEXT NOT NULL DEFAULT 'guest',
			teacher_id INTEGER,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')))`,

		`CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT NOT NULL UNIQUE,
			user_id INTEGER NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
			expires_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token)`,

		`CREATE TABLE IF NOT EXISTS system_config (
			id INTEGER PRIMARY KEY CHECK (id=1),
			school_name TEXT NOT NULL DEFAULT '',
			school_days TEXT NOT NULL DEFAULT '1,2,3,4,5',
			noon_rest TEXT NOT NULL DEFAULT '12:00-14:00',
			school_over TEXT NOT NULL DEFAULT '17:40')`,

		`CREATE TABLE IF NOT EXISTS periods (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			period_index INTEGER NOT NULL UNIQUE,
			start_time TEXT NOT NULL,
			end_time TEXT NOT NULL,
			period_type TEXT NOT NULL DEFAULT 'morning')`,

		`CREATE TABLE IF NOT EXISTS grades (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			sort_order INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1)`,

		`CREATE TABLE IF NOT EXISTS classes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			grade_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			class_no INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1)`,
		`CREATE INDEX IF NOT EXISTS idx_classes_grade ON classes(grade_id)`,

		`CREATE TABLE IF NOT EXISTS subjects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			subject_type TEXT NOT NULL DEFAULT 'main',
			sort_order INTEGER NOT NULL DEFAULT 0,
			is_default INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1)`,

		`CREATE TABLE IF NOT EXISTS teachers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			phone TEXT NOT NULL DEFAULT '',
			is_class_teacher INTEGER NOT NULL DEFAULT 0,
			weekly_hour_limit INTEGER NOT NULL DEFAULT 20,
			allow_evening INTEGER NOT NULL DEFAULT 1,
			remark TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1)`,

		`CREATE TABLE IF NOT EXISTS teacher_subjects (
			teacher_id INTEGER NOT NULL,
			subject_id INTEGER NOT NULL,
			PRIMARY KEY (teacher_id, subject_id))`,

		`CREATE TABLE IF NOT EXISTS assignments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			class_id INTEGER NOT NULL,
			subject_id INTEGER NOT NULL,
			teacher_id INTEGER NOT NULL,
			weekly_hours INTEGER NOT NULL DEFAULT 1,
			prefer_morning INTEGER NOT NULL DEFAULT 0)`,
		`CREATE INDEX IF NOT EXISTS idx_assign_class ON assignments(class_id)`,
		`CREATE INDEX IF NOT EXISTS idx_assign_teacher ON assignments(teacher_id)`,

		`CREATE TABLE IF NOT EXISTS rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			rule_type TEXT NOT NULL,
			priority INTEGER NOT NULL DEFAULT 50,
			enabled INTEGER NOT NULL DEFAULT 1,
			is_default INTEGER NOT NULL DEFAULT 0,
			params TEXT NOT NULL DEFAULT '{}',
			description TEXT NOT NULL DEFAULT '')`,

		`CREATE TABLE IF NOT EXISTS timetable (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			class_id INTEGER NOT NULL,
			day_index INTEGER NOT NULL,
			period_index INTEGER NOT NULL,
			subject_id INTEGER NOT NULL,
			teacher_id INTEGER NOT NULL,
			source TEXT NOT NULL DEFAULT 'auto',
			updated_at TEXT NOT NULL DEFAULT (datetime('now','localtime')))`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_tt_unique ON timetable(class_id, day_index, period_index)`,
		`CREATE INDEX IF NOT EXISTS idx_tt_teacher ON timetable(teacher_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tt_class ON timetable(class_id)`,

		`CREATE TABLE IF NOT EXISTS logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			username TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL DEFAULT '',
			detail TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')))`,

		`CREATE TABLE IF NOT EXISTS backups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			filename TEXT NOT NULL,
			file_size INTEGER NOT NULL DEFAULT 0,
			backup_type TEXT NOT NULL DEFAULT 'manual',
			created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')))`,
	}
	for _, s := range stmts {
		if _, err := DB.Exec(s); err != nil {
			return fmt.Errorf("建表失败: %w", err)
		}
	}
	return nil
}

// Now 返回本地时间字符串
func Now() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// Close 关闭数据库连接（恢复操作时使用）
func Close() {
	if DB != nil {
		DB.Close()
		DB = nil
	}
}
