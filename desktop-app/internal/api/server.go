// Package api 实现 HTTP API 路由与通用处理
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"school-scheduler/internal/auth"
	"school-scheduler/internal/store"
)

type ctxKey int

const userKey ctxKey = 0

// Server HTTP 服务
type Server struct {
	mux *http.ServeMux
}

// NewServer 创建 API 服务器并注册全部路由
func NewServer() *Server {
	s := &Server{mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler 返回带会话中间件的处理器
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 会话中间件
		if c, err := r.Cookie("sas_token"); err == nil && c.Value != "" {
			if u, err := auth.UserByToken(c.Value); err == nil {
				ctx := context.WithValue(r.Context(), userKey, u)
				r = r.WithContext(ctx)
			}
		}
		s.mux.ServeHTTP(w, r)
	})
}

// CurrentUser 获取当前登录用户（可能为 nil）
func CurrentUser(r *http.Request) *auth.User {
	u, _ := r.Context().Value(userKey).(*auth.User)
	return u
}

/* ---------- 响应/请求工具 ---------- */

// writeJSON 写 JSON
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ok 成功响应
func ok(w http.ResponseWriter, data interface{}) {
	writeJSON(w, 200, map[string]interface{}{"success": true, "data": data})
}

// okMsg 成功响应（带 message）
func okMsg(w http.ResponseWriter, msg string, data interface{}) {
	writeJSON(w, 200, map[string]interface{}{"success": true, "message": msg, "data": data})
}

// fail 失败响应
func fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]interface{}{"success": false, "message": msg})
}

// unauthorized 未登录
func unauthorized(w http.ResponseWriter) {
	fail(w, 401, "未登录或登录已过期")
}

// forbidden 无权限
func forbidden(w http.ResponseWriter) {
	fail(w, 403, "无权限执行此操作")
}

// bindJSON 解析请求体
func bindJSON(r *http.Request, v interface{}) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

/* ---------- 权限检查 ---------- */

// requireLogin 检查登录
func requireLogin(w http.ResponseWriter, r *http.Request) *auth.User {
	u := CurrentUser(r)
	if u == nil {
		unauthorized(w)
		return nil
	}
	return u
}

// requirePerm 检查功能权限
func requirePerm(w http.ResponseWriter, r *http.Request, perm string) *auth.User {
	u := requireLogin(w, r)
	if u == nil {
		return nil
	}
	switch perm {
	case "view":
		if !auth.CanView(u.Role) {
			forbidden(w)
			return nil
		}
	case "edit":
		if !auth.CanEdit(u.Role) {
			forbidden(w)
			return nil
		}
	case "schedule":
		if !auth.CanSchedule(u.Role) {
			forbidden(w)
			return nil
		}
	case "export":
		if !auth.CanExport(u.Role) {
			forbidden(w)
			return nil
		}
	case "backup":
		if !auth.CanManageBackup(u.Role) {
			forbidden(w)
			return nil
		}
	case "users":
		if !auth.CanManageUsers(u.Role) {
			forbidden(w)
			return nil
		}
	}
	return u
}

/* ---------- 操作日志 ---------- */

// AddLog 记录操作日志
func AddLog(u *auth.User, action, detail string) {
	username := ""
	userID := 0
	if u != nil {
		username = u.Username
		userID = u.ID
	}
	store.SysDB.Exec("INSERT INTO logs (user_id, username, action, detail) VALUES (?,?,?,?)",
		userID, username, action, detail)
}

// pathID 从路径参数取 int 型 ID
func pathID(r *http.Request, name string) (int, bool) {
	v := r.PathValue(name)
	if v == "" {
		return 0, false
	}
	id, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return id, true
}

// IsNoRows 判断是否为无记录错误
func IsNoRows(err error) bool { return err == sql.ErrNoRows }

// itoa int 转字符串
func itoa(i int) string { return strconv.Itoa(i) }

// btoi bool 转 0/1
func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
