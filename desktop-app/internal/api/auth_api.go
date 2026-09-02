package api

import (
	"net/http"

	"school-scheduler/internal/auth"
	"school-scheduler/internal/store"
)

const sessionCookie = "sas_token"

// userJSON 用户 JSON 输出
type userJSON struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	RealName  string `json:"real_name"`
	Role      string `json:"role"`
	RoleName  string `json:"role_name"`
	TeacherID int    `json:"teacher_id"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

func toUserJSON(u *auth.User) userJSON {
	uid := 0
	if u.TeacherID.Valid {
		uid = int(u.TeacherID.Int64)
	}
	return userJSON{
		ID: u.ID, Username: u.Username, RealName: u.RealName,
		Role: u.Role, RoleName: auth.RoleName(u.Role),
		TeacherID: uid, Enabled: u.Enabled, CreatedAt: u.CreatedAt,
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := bindJSON(r, &body); err != nil {
		fail(w, 400, "请求参数错误")
		return
	}
	u, err := auth.Authenticate(body.Username, body.Password)
	if err != nil {
		fail(w, 401, err.Error())
		return
	}
	token, err := auth.CreateSession(u.ID)
	if err != nil {
		fail(w, 500, "创建会话失败")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/",
		MaxAge: 7 * 86400, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	AddLog(u, "login", "用户登录系统")
	ok(w, toUserJSON(u))
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		auth.DestroySession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	ok(w, nil)
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	u := requireLogin(w, r)
	if u == nil {
		return
	}
	ok(w, toUserJSON(u))
}

func handleChangePassword(w http.ResponseWriter, r *http.Request) {
	u := requireLogin(w, r)
	if u == nil {
		return
	}
	var body struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := bindJSON(r, &body); err != nil || body.NewPassword == "" {
		fail(w, 400, "参数错误")
		return
	}
	if len(body.NewPassword) < 6 {
		fail(w, 400, "新密码至少6位")
		return
	}
	var hash string
	if err := store.SysDB.QueryRow("SELECT password_hash FROM users WHERE id=?", u.ID).Scan(&hash); err != nil {
		fail(w, 500, "查询用户失败")
		return
	}
	if !auth.CheckPassword(hash, body.OldPassword) {
		fail(w, 400, "原密码错误")
		return
	}
	nh, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		fail(w, 500, "加密失败")
		return
	}
	store.SysDB.Exec("UPDATE users SET password_hash=? WHERE id=?", nh, u.ID)
	AddLog(u, "update", "修改个人密码")
	okMsg(w, "密码修改成功", nil)
}

/* ---------- 账号管理（仅超级管理员） ---------- */

func handleUsers(w http.ResponseWriter, r *http.Request) {
	if requirePerm(w, r, "users") == nil {
		return
	}
	switch r.Method {
	case "GET":
		rows, err := store.SysDB.Query(
			"SELECT id, username, real_name, role, COALESCE(teacher_id,0), enabled, created_at FROM users ORDER BY id")
		if err != nil {
			fail(w, 500, "查询失败")
			return
		}
		defer rows.Close()
		list := []userJSON{}
		for rows.Next() {
			var uj userJSON
			if err := rows.Scan(&uj.ID, &uj.Username, &uj.RealName, &uj.Role, &uj.TeacherID, &uj.Enabled, &uj.CreatedAt); err == nil {
				uj.RoleName = auth.RoleName(uj.Role)
				list = append(list, uj)
			}
		}
		ok(w, list)
	case "POST":
		var body struct {
			Username  string `json:"username"`
			RealName  string `json:"real_name"`
			Role      string `json:"role"`
			Password  string `json:"password"`
			TeacherID *int   `json:"teacher_id"`
		}
		if err := bindJSON(r, &body); err != nil || body.Username == "" {
			fail(w, 400, "请填写用户名")
			return
		}
		// 权限已统一：所有账号拥有完整功能，角色标识固定为 super
		body.Role = "super"
		if body.Password == "" {
			body.Password = "123456"
		}
		hash, err := auth.HashPassword(body.Password)
		if err != nil {
			fail(w, 500, "加密失败")
			return
		}
		var tid interface{}
		if body.TeacherID != nil && *body.TeacherID > 0 {
			tid = *body.TeacherID
		}
		res, err := store.SysDB.Exec(
			"INSERT INTO users (username, password_hash, real_name, role, teacher_id) VALUES (?,?,?,?,?)",
			body.Username, hash, body.RealName, body.Role, tid)
		if err != nil {
			fail(w, 400, "用户名已存在或参数错误")
			return
		}
		id, _ := res.LastInsertId()
		AddLog(CurrentUser(r), "create", "新增账号 "+body.Username)
		ok(w, map[string]interface{}{"id": id})
	}
}

func handleUserUpdate(w http.ResponseWriter, r *http.Request) {
	if requirePerm(w, r, "users") == nil {
		return
	}
	id, okID := pathID(r, "id")
	if !okID {
		fail(w, 400, "参数错误")
		return
	}
	me := CurrentUser(r)
	if r.Method == "DELETE" {
		if id == me.ID {
			fail(w, 400, "不能删除当前登录账号")
			return
		}
		store.SysDB.Exec("DELETE FROM users WHERE id=?", id)
		AddLog(me, "delete", "删除账号 #"+itoa(id))
		okMsg(w, "删除成功", nil)
		return
	}
	var body struct {
		RealName  string `json:"real_name"`
		Role      string `json:"role"`
		Password  string `json:"password"`
		TeacherID *int   `json:"teacher_id"`
		Enabled   *bool  `json:"enabled"`
	}
	if err := bindJSON(r, &body); err != nil {
		fail(w, 400, "参数错误")
		return
	}
	if id == me.ID && body.Enabled != nil && !*body.Enabled {
		fail(w, 400, "不能禁用当前登录账号")
		return
	}
	if body.Enabled != nil {
		if _, err := store.SysDB.Exec("UPDATE users SET enabled=? WHERE id=?", btoi(*body.Enabled), id); err != nil {
			fail(w, 500, "更新失败")
			return
		}
	}
	if body.RealName != "" {
		// 权限已统一：编辑姓名时角色固定为 super，避免遗留角色标识产生差异
		store.SysDB.Exec("UPDATE users SET real_name=?, role='super' WHERE id=?", body.RealName, id)
	}
	if body.Password != "" {
		if len(body.Password) < 6 {
			fail(w, 400, "密码至少6位")
			return
		}
		hash, _ := auth.HashPassword(body.Password)
		store.SysDB.Exec("UPDATE users SET password_hash=? WHERE id=?", hash, id)
	}
	if body.TeacherID != nil {
		var tid interface{}
		if *body.TeacherID > 0 {
			tid = *body.TeacherID
		}
		store.SysDB.Exec("UPDATE users SET teacher_id=? WHERE id=?", tid, id)
	}
	AddLog(me, "update", "修改账号 #"+itoa(id))
	okMsg(w, "保存成功", nil)
}

func handleResetPassword(w http.ResponseWriter, r *http.Request) {
	if requirePerm(w, r, "users") == nil {
		return
	}
	id, okID := pathID(r, "id")
	if !okID {
		fail(w, 400, "参数错误")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := bindJSON(r, &body); err != nil || body.Password == "" {
		fail(w, 400, "请填写新密码")
		return
	}
	if len(body.Password) < 6 {
		fail(w, 400, "密码至少6位")
		return
	}
	hash, _ := auth.HashPassword(body.Password)
	store.SysDB.Exec("UPDATE users SET password_hash=? WHERE id=?", hash, id)
	AddLog(CurrentUser(r), "reset", "重置账号 #"+itoa(id)+" 密码")
	okMsg(w, "密码已重置", nil)
}
