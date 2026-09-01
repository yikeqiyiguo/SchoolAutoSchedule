package api

import (
	"net/http"
	"strconv"

	"school-scheduler/internal/store"
)

func handleLogActions(w http.ResponseWriter, r *http.Request) {
	if requirePerm(w, r, "view") == nil {
		return
	}
	ok(w, store.ActionNames)
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	if requirePerm(w, r, "view") == nil {
		return
	}
	page := strToInt(r.URL.Query().Get("page"), 1)
	perPage := strToInt(r.URL.Query().Get("per_page"), 20)
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	where := ""
	args := []interface{}{}
	if a := r.URL.Query().Get("action"); a != "" {
		where += " AND action=?"
		args = append(args, a)
	}
	if k := r.URL.Query().Get("keyword"); k != "" {
		where += " AND (username LIKE ? OR detail LIKE ?)"
		kw := "%" + k + "%"
		args = append(args, kw, kw)
	}
	var total int
	store.DB.QueryRow("SELECT COUNT(*) FROM logs WHERE 1=1"+where, args...).Scan(&total)
	pages := (total + perPage - 1) / perPage
	if pages < 1 {
		pages = 1
	}
	offset := (page - 1) * perPage
	rows, err := store.DB.Query(
		"SELECT id, username, action, detail, created_at FROM logs WHERE 1=1"+where+
			" ORDER BY id DESC LIMIT ? OFFSET ?",
		append(args, perPage, offset)...)
	if err != nil {
		fail(w, 500, "查询失败")
		return
	}
	defer rows.Close()
	list := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var username, action, detail, created string
		if err := rows.Scan(&id, &username, &action, &detail, &created); err == nil {
			an := action
			if v, ok := store.ActionNames[action]; ok {
				an = v
			}
			list = append(list, map[string]interface{}{
				"id": id, "username": username, "action": action, "action_name": an,
				"detail": detail, "created_at": created,
			})
		}
	}
	writeJSON(w, 200, map[string]interface{}{
		"success": true, "data": list, "page": page, "pages": pages, "total": total,
	})
}

// strToInt 字符串转 int
func strToInt(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
