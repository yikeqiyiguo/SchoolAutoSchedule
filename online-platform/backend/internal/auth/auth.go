// Package auth 提供密码哈希、会话管理
package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	"school-scheduler/internal/store"
)

// User 用户模型
type User struct {
	ID        int
	Username  string
	RealName  string
	Role      string
	TeacherID sql.NullInt64
	Enabled   bool
	CreatedAt string
}

// ErrNotFound 未找到
var ErrNotFound = errors.New("not found")

// HashPassword 生成 bcrypt 密码哈希
func HashPassword(pwd string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	return string(h), err
}

// CheckPassword 校验密码
func CheckPassword(hash, pwd string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pwd)) == nil
}

// GetUserByUsername 按用户名查询用户
func GetUserByUsername(username string) (*User, error) {
	row := store.SysDB.QueryRow(
		"SELECT id, username, real_name, role, COALESCE(teacher_id,0), enabled, created_at FROM users WHERE username=?",
		username)
	return scanUser(row)
}

// GetUserByID 按 ID 查询
func GetUserByID(id int) (*User, error) {
	row := store.SysDB.QueryRow(
		"SELECT id, username, real_name, role, COALESCE(teacher_id,0), enabled, created_at FROM users WHERE id=?",
		id)
	return scanUser(row)
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	var teacherID int
	if err := row.Scan(&u.ID, &u.Username, &u.RealName, &u.Role, &teacherID, &u.Enabled, &u.CreatedAt); err != nil {
		return nil, err
	}
	if teacherID > 0 {
		u.TeacherID = sql.NullInt64{Int64: int64(teacherID), Valid: true}
	}
	return &u, nil
}

// Authenticate 验证用户名密码
func Authenticate(username, password string) (*User, error) {
	var hash string
	err := store.SysDB.QueryRow("SELECT password_hash FROM users WHERE username=? AND enabled=1", username).Scan(&hash)
	if err != nil {
		return nil, errors.New("用户名或密码错误")
	}
	if !CheckPassword(hash, password) {
		return nil, errors.New("用户名或密码错误")
	}
	return GetUserByUsername(username)
}

// CreateSession 创建会话并返回 token
func CreateSession(userID int) (string, error) {
	token := newToken()
	expires := time.Now().AddDate(0, 0, 7).Format("2006-01-02 15:04:05")
	_, err := store.SysDB.Exec(
		"INSERT INTO sessions (token, user_id, expires_at) VALUES (?,?,?)",
		token, userID, expires)
	if err != nil {
		return "", err
	}
	return token, nil
}

// UserByToken 根据会话 token 获取用户
func UserByToken(token string) (*User, error) {
	if token == "" {
		return nil, sql.ErrNoRows
	}
	var userID int
	var expires string
	err := store.SysDB.QueryRow(
		"SELECT user_id, expires_at FROM sessions WHERE token=?", token).Scan(&userID, &expires)
	if err != nil {
		return nil, err
	}
	if t, err := time.Parse("2006-01-02 15:04:05", expires); err == nil && t.Before(time.Now()) {
		store.SysDB.Exec("DELETE FROM sessions WHERE token=?", token)
		return nil, sql.ErrNoRows
	}
	u, err := GetUserByID(userID)
	if err != nil || !u.Enabled {
		return nil, sql.ErrNoRows
	}
	return u, nil
}

// DestroySession 删除会话
func DestroySession(token string) {
	store.SysDB.Exec("DELETE FROM sessions WHERE token=?", token)
}

func newToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// RoleName 角色显示名
// 系统权限已统一：所有登录用户均拥有完整功能权限，对外统一显示为系统管理员
func RoleName(role string) string {
	return "系统管理员"
}

// CanManageUsers 是否可管理账号（统一权限：登录即可）
func CanManageUsers(role string) bool { return true }

// CanView 是否可查看课表
func CanView(role string) bool { return true }

// CanEdit 是否可编辑基础数据
func CanEdit(role string) bool { return true }

// CanSchedule 是否可执行排课
func CanSchedule(role string) bool { return true }

// CanExport 是否可导出
func CanExport(role string) bool { return true }

// CanManageBackup 是否可备份恢复
func CanManageBackup(role string) bool { return true }
