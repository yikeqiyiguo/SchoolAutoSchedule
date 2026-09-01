package api

import (
	"net/http"
	"strconv"
	"strings"

	"school-scheduler/internal/auth"
	"school-scheduler/internal/store"
)

func handleGetSystem(w http.ResponseWriter, r *http.Request) {
	if requirePerm(w, r, "view") == nil {
		return
	}
	var schoolName, schoolDays, noonRest, schoolOver string
	err := store.DB.QueryRow(
		"SELECT school_name, school_days, noon_rest, school_over FROM system_config WHERE id=1").
		Scan(&schoolName, &schoolDays, &noonRest, &schoolOver)
	if err != nil {
		schoolDays = "1,2,3,4,5"
	}
	days := []int{}
	for _, d := range strings.Split(schoolDays, ",") {
		if n, e := strconv.Atoi(strings.TrimSpace(d)); e == nil && n >= 1 && n <= 7 {
			days = append(days, n)
		}
	}
	ok(w, map[string]interface{}{
		"school_name": schoolName,
		"school_days": days,
		"noon_rest":   noonRest,
		"school_over": schoolOver,
	})
}

func handlePutSystem(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "edit")
	if u == nil {
		return
	}
	var body struct {
		SchoolName string `json:"school_name"`
		SchoolDays []int  `json:"school_days"`
		NoonRest   string `json:"noon_rest"`
		SchoolOver string `json:"school_over"`
	}
	if err := bindJSON(r, &body); err != nil {
		fail(w, 400, "参数错误")
		return
	}
	if len(body.SchoolDays) == 0 {
		fail(w, 400, "请至少选择一个上课日")
		return
	}
	parts := []string{}
	for _, d := range body.SchoolDays {
		parts = append(parts, itoa(d))
	}
	store.DB.Exec("UPDATE system_config SET school_name=?, school_days=?, noon_rest=?, school_over=? WHERE id=1",
		body.SchoolName, strings.Join(parts, ","), body.NoonRest, body.SchoolOver)
	AddLog(u, "update", "修改系统配置")
	okMsg(w, "保存成功", nil)
}

func handleGetPeriods(w http.ResponseWriter, r *http.Request) {
	if requirePerm(w, r, "view") == nil {
		return
	}
	rows, err := store.DB.Query("SELECT id, period_index, start_time, end_time, period_type FROM periods ORDER BY period_index")
	if err != nil {
		fail(w, 500, "查询失败")
		return
	}
	defer rows.Close()
	list := []map[string]interface{}{}
	for rows.Next() {
		var id, idx int
		var st, et, pt string
		if err := rows.Scan(&id, &idx, &st, &et, &pt); err == nil {
			list = append(list, map[string]interface{}{
				"id": id, "period_index": idx, "start_time": st, "end_time": et, "period_type": pt,
			})
		}
	}
	ok(w, list)
}

func handlePutPeriods(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "edit")
	if u == nil {
		return
	}
	var body struct {
		Periods []struct {
			PeriodIndex int    `json:"period_index"`
			StartTime   string `json:"start_time"`
			EndTime     string `json:"end_time"`
			PeriodType  string `json:"period_type"`
		} `json:"periods"`
	}
	if err := bindJSON(r, &body); err != nil || len(body.Periods) == 0 {
		fail(w, 400, "参数错误")
		return
	}
	tx, err := store.DB.Begin()
	if err != nil {
		fail(w, 500, "事务失败")
		return
	}
	tx.Exec("DELETE FROM periods")
	for _, p := range body.Periods {
		if _, err := tx.Exec("INSERT INTO periods (period_index, start_time, end_time, period_type) VALUES (?,?,?,?)",
			p.PeriodIndex, p.StartTime, p.EndTime, p.PeriodType); err != nil {
			tx.Rollback()
			fail(w, 400, "时段数据不合法")
			return
		}
	}
	tx.Commit()
	AddLog(u, "update", "修改课时时段配置")
	okMsg(w, "保存成功", nil)
}

func handleMeta(w http.ResponseWriter, r *http.Request) {
	if requirePerm(w, r, "view") == nil {
		return
	}
	ok(w, map[string]interface{}{
		"rule_types":       store.RuleTypes,
		"period_type_names": store.PeriodTypeNames,
		"action_names":      store.ActionNames,
	})
}

// RoleNames 供导出使用
var RoleNames = map[string]string{}

func init() {
	for _, role := range []string{"super", "operator", "teacher", "guest"} {
		RoleNames[role] = auth.RoleName(role)
	}
}
