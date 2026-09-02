// Package store 负责 SQLite 数据库连接、建表与种子数据
// 多学校架构：system.db 存全局账号/会话/日志/学校列表；schools/school_N.db 存各校独立业务数据
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "modernc.org/sqlite"

	"school-scheduler/internal/config"
)

var (
	// SysDB 系统库：users / sessions / logs / schools / meta
	SysDB *sql.DB
	// DB 当前学校业务库（所有业务 API 直接使用）
	DB *sql.DB

	// 当前学校信息
	CurrentSchoolID   int
	CurrentSchoolName string
	currentDBFile     string
)

/* ---------- 入口 ---------- */

// Open 打开系统库与当前学校库（首次运行自动创建默认学校；检测到旧版单库自动迁移）
func Open() error {
	sys, err := openDBAt(config.SysDBPath)
	if err != nil {
		return err
	}
	SysDB = sys
	if err := initSystemSchema(sys); err != nil {
		return fmt.Errorf("系统库建表失败: %w", err)
	}
	if err := sysSeedIfEmpty(sys); err != nil {
		return err
	}
	// 学校列表为空：迁移旧库或创建默认学校
	var n int
	if err := sys.QueryRow("SELECT COUNT(*) FROM schools").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if fileExists(config.DBPath) {
			if err := migrateLegacy(config.DBPath); err != nil {
				return fmt.Errorf("旧数据迁移失败: %w", err)
			}
		} else {
			if _, err := CreateSchool("默认学校"); err != nil {
				return err
			}
		}
	}
	// 打开当前学校（meta 记忆上次选择）
	id := 0
	var cur string
	if err := sys.QueryRow("SELECT v FROM meta WHERE k='current_school_id'").Scan(&cur); err == nil {
		id, _ = strconv.Atoi(cur)
	}
	if id == 0 || !schoolExists(id) {
		if err := sys.QueryRow("SELECT MIN(id) FROM schools").Scan(&id); err != nil || id == 0 {
			return fmt.Errorf("无可用学校")
		}
		setMeta("current_school_id", fmt.Sprintf("%d", id))
	}
	return openCurrentSchool(id)
}

// 重新打开当前学校（备份恢复后使用）
func ReopenCurrentSchool() error {
	DB.Close()
	DB = nil
	return openCurrentSchool(CurrentSchoolID)
}

// CurrentDBFile 当前学校库文件路径
func CurrentDBFile() string { return currentDBFile }

/* ---------- 系统库 ---------- */

func initSystemSchema(db *sql.DB) error {
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
		`CREATE TABLE IF NOT EXISTS logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			username TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL DEFAULT '',
			detail TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')))`,
		`CREATE TABLE IF NOT EXISTS schools (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			db_file TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')))`,
		`CREATE TABLE IF NOT EXISTS meta (
			k TEXT PRIMARY KEY,
			v TEXT NOT NULL DEFAULT '')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("建表失败: %w", err)
		}
	}
	return nil
}

func setMeta(k, v string) {
	SysDB.Exec("INSERT INTO meta (k, v) VALUES (?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v", k, v)
}

func getMeta(k string) string {
	var v string
	if err := SysDB.QueryRow("SELECT v FROM meta WHERE k=?", k).Scan(&v); err != nil {
		return ""
	}
	return v
}

/* ---------- 学校库 ---------- */

func initSchoolSchema(db *sql.DB) error {
	stmts := []string{
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
		`CREATE TABLE IF NOT EXISTS backups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			filename TEXT NOT NULL,
			file_size INTEGER NOT NULL DEFAULT 0,
			backup_type TEXT NOT NULL DEFAULT 'manual',
			created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')))`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("建表失败: %w", err)
		}
	}
	return nil
}

func openDBAt(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite 单写者
	db.SetMaxIdleConns(1)
	for _, p := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=8000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}
	return db, nil
}

// openCurrentSchool 打开指定学校并记为当前
func openCurrentSchool(id int) error {
	var name, file string
	if err := SysDB.QueryRow("SELECT name, db_file FROM schools WHERE id=?", id).Scan(&name, &file); err != nil {
		return fmt.Errorf("学校不存在: %w", err)
	}
	db, err := openDBAt(filepath.Join(config.SchoolsDir, file))
	if err != nil {
		return err
	}
	if err := initSchoolSchema(db); err != nil {
		db.Close()
		return err
	}
	if err := schoolSeedIfEmpty(db); err != nil {
		db.Close()
		return err
	}
	DB = db
	CurrentSchoolID = id
	CurrentSchoolName = name
	currentDBFile = filepath.Join(config.SchoolsDir, file)
	return nil
}

/* ---------- 学校管理 ---------- */

// School 学校记录
type School struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// ListSchools 全部学校
func ListSchools() []School {
	rows, err := SysDB.Query("SELECT id, name, created_at FROM schools ORDER BY id")
	if err != nil {
		return []School{}
	}
	defer rows.Close()
	out := []School{}
	for rows.Next() {
		var s School
		if err := rows.Scan(&s.ID, &s.Name, &s.CreatedAt); err == nil {
			out = append(out, s)
		}
	}
	return out
}

func schoolExists(id int) bool {
	var n int
	SysDB.QueryRow("SELECT COUNT(*) FROM schools WHERE id=?", id).Scan(&n)
	return n > 0
}

// CreateSchool 新建学校（独立数据库文件），返回学校 ID
func CreateSchool(name string) (int, error) {
	if name == "" {
		return 0, fmt.Errorf("学校名称不能为空")
	}
	res, err := SysDB.Exec("INSERT INTO schools (name, db_file) VALUES (?,?)", name, "")
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	file := fmt.Sprintf("school_%d.db", id)
	if _, err := SysDB.Exec("UPDATE schools SET db_file=? WHERE id=?", file, id); err != nil {
		return 0, err
	}
	// 初始化学校库（建表+种子），随后关闭，待切换时再打开
	db, err := openDBAt(filepath.Join(config.SchoolsDir, file))
	if err != nil {
		return 0, err
	}
	defer db.Close()
	if err := initSchoolSchema(db); err != nil {
		return 0, err
	}
	if err := schoolSeedIfEmpty(db); err != nil {
		return 0, err
	}
	return int(id), nil
}

// RenameSchool 重命名学校（同步学校库内 system_config.school_name）
func RenameSchool(id int, name string) error {
	if name == "" {
		return fmt.Errorf("学校名称不能为空")
	}
	if _, err := SysDB.Exec("UPDATE schools SET name=? WHERE id=?", name, id); err != nil {
		return err
	}
	if id == CurrentSchoolID {
		CurrentSchoolName = name
		DB.Exec("UPDATE system_config SET school_name=? WHERE id=1", name)
	}
	return nil
}

// DeleteSchool 删除学校（不可删除当前学校/最后一所）
func DeleteSchool(id int) error {
	var n int
	SysDB.QueryRow("SELECT COUNT(*) FROM schools").Scan(&n)
	if n <= 1 {
		return fmt.Errorf("至少保留一所学校")
	}
	if id == CurrentSchoolID {
		return fmt.Errorf("不能删除当前正在使用的学校，请先切换")
	}
	var file string
	if err := SysDB.QueryRow("SELECT db_file FROM schools WHERE id=?", id).Scan(&file); err != nil {
		return fmt.Errorf("学校不存在")
	}
	os.Remove(filepath.Join(config.SchoolsDir, file))
	os.Remove(filepath.Join(config.SchoolsDir, file) + "-wal")
	os.Remove(filepath.Join(config.SchoolsDir, file) + "-shm")
	_, err := SysDB.Exec("DELETE FROM schools WHERE id=?", id)
	return err
}

// SwitchSchool 切换当前学校（关闭当前库，打开目标库）
func SwitchSchool(id int) error {
	if !schoolExists(id) {
		return fmt.Errorf("学校不存在")
	}
	if DB != nil {
		DB.Close()
		DB = nil
	}
	if err := openCurrentSchool(id); err != nil {
		return err
	}
	setMeta("current_school_id", fmt.Sprintf("%d", id))
	return nil
}

/* ---------- 旧版单库迁移 ---------- */

// migrateLegacy 将旧版 scheduler.db 拆分为 system.db + 默认学校库
func migrateLegacy(oldPath string) error {
	if err := migrateLegacyInner(oldPath); err != nil {
		// 回滚半迁移状态，保证下次启动可重试
		SysDB.Exec("DELETE FROM schools")
		SysDB.Exec("DELETE FROM users")
		SysDB.Exec("DELETE FROM sessions")
		SysDB.Exec("DELETE FROM logs")
		matches, _ := filepath.Glob(filepath.Join(config.SchoolsDir, "school_*.db"))
		for _, m := range matches {
			os.Remove(m)
			os.Remove(m + "-wal")
			os.Remove(m + "-shm")
		}
		return err
	}
	return nil
}

func migrateLegacyInner(oldPath string) error {
	// 1) 创建默认学校记录
	res, err := SysDB.Exec("INSERT INTO schools (name, db_file) VALUES (?,?)", "默认学校", "")
	if err != nil {
		return err
	}
	sid, _ := res.LastInsertId()
	file := fmt.Sprintf("school_%d.db", sid)
	if _, err := SysDB.Exec("UPDATE schools SET db_file=? WHERE id=?", file, sid); err != nil {
		return err
	}
	schoolPath := filepath.Join(config.SchoolsDir, file)

	// 2) 学校库：建表后从旧库复制业务数据
	sdb, err := openDBAt(schoolPath)
	if err != nil {
		return err
	}
	defer sdb.Close()
	if err := initSchoolSchema(sdb); err != nil {
		return err
	}
	if _, err := sdb.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS old", oldPath)); err != nil {
		return fmt.Errorf("attach: %w", err)
	}
	for _, t := range []string{
		"system_config", "periods", "grades", "classes", "subjects", "teachers",
		"teacher_subjects", "assignments", "rules", "timetable", "backups",
	} {
		var has int
		if err := sdb.QueryRow("SELECT COUNT(*) FROM old.sqlite_master WHERE type='table' AND name=?", t).Scan(&has); err != nil || has == 0 {
			continue // 旧库缺表则跳过
		}
		if _, err := sdb.Exec(fmt.Sprintf("INSERT INTO %s SELECT * FROM old.%s", t, t)); err != nil {
			sdb.Exec("DETACH DATABASE old")
			return fmt.Errorf("复制表 %s: %w", t, err)
		}
	}
	sdb.Exec("DETACH DATABASE old")

	// 3) 系统库：复制账号/会话/日志（先清掉启动时种子的默认账号，以旧库数据为准）
	if _, err := SysDB.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS old", oldPath)); err != nil {
		return fmt.Errorf("attach: %w", err)
	}
	for _, t := range []string{"users", "sessions", "logs"} {
		SysDB.Exec("DELETE FROM " + t)
		var has int
		if err := SysDB.QueryRow("SELECT COUNT(*) FROM old.sqlite_master WHERE type='table' AND name=?", t).Scan(&has); err != nil || has == 0 {
			continue // 旧库缺表则跳过
		}
		if _, err := SysDB.Exec(fmt.Sprintf("INSERT INTO %s SELECT * FROM old.%s", t, t)); err != nil {
			SysDB.Exec("DETACH DATABASE old")
			return fmt.Errorf("复制表 %s: %w", t, err)
		}
	}
	SysDB.Exec("DETACH DATABASE old")

	// 4) 旧库改名保留
	os.Rename(oldPath, oldPath+".migrated.bak")
	os.Remove(oldPath + "-wal")
	os.Remove(oldPath + "-shm")
	return nil
}

/* ---------- 工具 ---------- */

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Now 返回本地时间字符串
func Now() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// Close 关闭全部连接（恢复操作时使用）
func Close() {
	if DB != nil {
		DB.Close()
		DB = nil
	}
	if SysDB != nil {
		SysDB.Close()
		SysDB = nil
	}
}
