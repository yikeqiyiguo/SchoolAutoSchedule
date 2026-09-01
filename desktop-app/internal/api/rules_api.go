package api

import (
	"encoding/json"
	"net/http"

	"school-scheduler/internal/store"
)

func handleRules(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "view")
	if u == nil {
		return
	}
	if r.Method == "GET" {
		rows, err := store.DB.Query("SELECT id, name, rule_type, priority, enabled, is_default, params, description FROM rules ORDER BY priority DESC, id")
		if err != nil {
			fail(w, 500, "查询失败")
			return
		}
		defer rows.Close()
		list := []map[string]interface{}{}
		for rows.Next() {
			var id, prio, isDefault int
			var name, rtype, params, desc string
			var enabled bool
			if err := rows.Scan(&id, &name, &rtype, &prio, &enabled, &isDefault, &params, &desc); err == nil {
				pm := map[string]interface{}{}
				json.Unmarshal([]byte(params), &pm)
				tn := rtype
				if m, ok := store.RuleTypes[rtype]; ok {
					tn = m.Name
				}
				list = append(list, map[string]interface{}{
					"id": id, "name": name, "rule_type": rtype, "rule_type_name": tn,
					"priority": prio, "enabled": enabled, "is_default": isDefault == 1,
					"params": pm, "description": desc,
				})
			}
		}
		ok(w, list)
		return
	}
	if requirePerm(w, r, "edit") == nil {
		return
	}
	var body struct {
		Name        string                 `json:"name"`
		RuleType    string                 `json:"rule_type"`
		Priority    int                    `json:"priority"`
		Description string                 `json:"description"`
		Params      map[string]interface{} `json:"params"`
	}
	if err := bindJSON(r, &body); err != nil || body.Name == "" {
		fail(w, 400, "请填写规则名称")
		return
	}
	if _, ok := store.RuleTypes[body.RuleType]; !ok {
		fail(w, 400, "规则类型不合法")
		return
	}
	if body.Params == nil {
		body.Params = map[string]interface{}{}
	}
	params, _ := json.Marshal(body.Params)
	if body.Priority == 0 {
		body.Priority = 50
	}
	res, err := store.DB.Exec("INSERT INTO rules (name, rule_type, priority, enabled, params, description) VALUES (?,?,?,1,?,?)",
		body.Name, body.RuleType, body.Priority, string(params), body.Description)
	if err != nil {
		fail(w, 500, "保存失败")
		return
	}
	id, _ := res.LastInsertId()
	AddLog(u, "create", "新增规则 "+body.Name)
	ok(w, map[string]interface{}{"id": id})
}

func handleRuleUpdate(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "edit")
	if u == nil {
		return
	}
	id, okID := pathID(r, "id")
	if !okID {
		fail(w, 400, "参数错误")
		return
	}
	if r.Method == "DELETE" {
		var isDefault bool
		store.DB.QueryRow("SELECT is_default FROM rules WHERE id=?", id).Scan(&isDefault)
		if isDefault {
			fail(w, 400, "系统内置规则不允许删除，可禁用")
			return
		}
		store.DB.Exec("DELETE FROM rules WHERE id=?", id)
		AddLog(u, "delete", "删除规则 #"+itoa(id))
		okMsg(w, "删除成功", nil)
		return
	}
	var body struct {
		Name        string                 `json:"name"`
		RuleType    string                 `json:"rule_type"`
		Priority    int                    `json:"priority"`
		Enabled     *bool                  `json:"enabled"`
		Description string                 `json:"description"`
		Params      map[string]interface{} `json:"params"`
	}
	if err := bindJSON(r, &body); err != nil {
		fail(w, 400, "参数错误")
		return
	}
	if body.Name != "" {
		store.DB.Exec("UPDATE rules SET name=? WHERE id=?", body.Name, id)
	}
	if body.RuleType != "" {
		if _, ok := store.RuleTypes[body.RuleType]; ok {
			store.DB.Exec("UPDATE rules SET rule_type=? WHERE id=?", body.RuleType, id)
		}
	}
	if body.Priority > 0 {
		store.DB.Exec("UPDATE rules SET priority=? WHERE id=?", body.Priority, id)
	}
	if body.Enabled != nil {
		store.DB.Exec("UPDATE rules SET enabled=? WHERE id=?", btoi(*body.Enabled), id)
	}
	if body.Description != "" {
		store.DB.Exec("UPDATE rules SET description=? WHERE id=?", body.Description, id)
	}
	if body.Params != nil {
		params, _ := json.Marshal(body.Params)
		store.DB.Exec("UPDATE rules SET params=? WHERE id=?", string(params), id)
	}
	AddLog(u, "update", "修改规则 #"+itoa(id))
	okMsg(w, "保存成功", nil)
}
