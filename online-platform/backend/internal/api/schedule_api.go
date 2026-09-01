package api

import (
	"net/http"
	"strconv"

	"school-scheduler/internal/auth"
	"school-scheduler/internal/sched"
	"school-scheduler/internal/store"
)

/* ---------- 排课总览 ---------- */

func handleScheduleOverview(w http.ResponseWriter, r *http.Request) {
	if requirePerm(w, r, "view") == nil {
		return
	}
	var periodCount int
	store.DB.QueryRow("SELECT COUNT(*) FROM periods").Scan(&periodCount)
	var totalClasses int
	store.DB.QueryRow("SELECT COUNT(*) FROM classes WHERE enabled=1").Scan(&totalClasses)

	rows, err := store.DB.Query(
		`SELECT c.id, g.name, c.name,
			(SELECT COALESCE(SUM(a.weekly_hours),0) FROM assignments a WHERE a.class_id=c.id),
			(SELECT COUNT(*) FROM timetable tt WHERE tt.class_id=c.id)
		 FROM classes c JOIN grades g ON c.grade_id=g.id WHERE c.enabled=1 ORDER BY g.sort_order, c.class_no`)
	if err != nil {
		fail(w, 500, "查询失败")
		return
	}
	defer rows.Close()
	classes := []map[string]interface{}{}
	for rows.Next() {
		var cid, need, have int
		var gname, cname string
		if err := rows.Scan(&cid, &gname, &cname, &need, &have); err != nil {
			continue
		}
		percent := 0
		if need > 0 {
			percent = have * 100 / need
		}
		classes = append(classes, map[string]interface{}{
			"class_id": cid, "grade_name": gname, "class_name": cname,
			"need": need, "have": have, "percent": percent,
		})
	}
	ok(w, map[string]interface{}{
		"total_classes":          totalClasses,
		"total_periods_per_week": periodCount * 5,
		"classes":                classes,
	})
}

/* ---------- 一键排课 ---------- */

func handleScheduleRun(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "schedule")
	if u == nil {
		return
	}
	var body struct {
		KeepManual bool `json:"keep_manual"`
	}
	bindJSON(r, &body)
	result, err := sched.Run(body.KeepManual)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	detail := "一键排课：" + itoa(result.Placed) + "节，耗时" + itoa(int(result.CostMS)) + "ms"
	AddLog(u, "schedule", detail)
	writeJSON(w, 200, map[string]interface{}{
		"success": result.Success, "message": result.Message,
		"cost_ms": result.CostMS, "placed": result.Placed,
		"conflict_count": len(result.Conflicts), "conflicts": result.Conflicts,
		"errors": result.Errors,
	})
}

/* ---------- 清空课表 ---------- */

func handleScheduleClear(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "schedule")
	if u == nil {
		return
	}
	var body struct {
		Type string `json:"type"`
	}
	bindJSON(r, &body)
	switch body.Type {
	case "all":
		store.DB.Exec("DELETE FROM timetable")
		AddLog(u, "schedule", "清空全部课表")
	default:
		store.DB.Exec("DELETE FROM timetable WHERE source='auto'")
		AddLog(u, "schedule", "清空自动排课课表")
	}
	okMsg(w, "课表已清空", nil)
}

/* ---------- 冲突检测 ---------- */

func handleScheduleConflicts(w http.ResponseWriter, r *http.Request) {
	if requirePerm(w, r, "schedule") == nil {
		return
	}
	ok(w, detectConflictsFromDB())
}

func detectConflictsFromDB() []map[string]interface{} {
	out := []map[string]interface{}{}
	// 教师撞课
	rows, err := store.DB.Query(
		`SELECT t.name, tt.day_index, tt.period_index, COUNT(*) FROM timetable tt
		 JOIN teachers t ON tt.teacher_id=t.id
		 GROUP BY tt.teacher_id, tt.day_index, tt.period_index HAVING COUNT(*)>1`)
	if err == nil {
		for rows.Next() {
			var name string
			var d, p, cnt int
			rows.Scan(&name, &d, &p, &cnt)
			out = append(out, map[string]interface{}{
				"type": "teacher_conflict", "level": "error",
				"message": "教师「" + name + "」" + dayName2(d) + "第" + itoa(p) + "节同时安排了" + itoa(cnt) + "节课",
			})
		}
		rows.Close()
	}
	// 教师超课时
	rows, err = store.DB.Query(
		`SELECT t.name, COUNT(tt.id), t.weekly_hour_limit FROM teachers t
		 LEFT JOIN timetable tt ON tt.teacher_id=t.id
		 WHERE t.enabled=1 GROUP BY t.id HAVING COUNT(tt.id) > t.weekly_hour_limit`)
	if err == nil {
		for rows.Next() {
			var name string
			var cnt, limit int
			rows.Scan(&name, &cnt, &limit)
			out = append(out, map[string]interface{}{
				"type": "hour_limit", "level": "warning",
				"message": "教师「" + name + "」本周" + itoa(cnt) + "节，超过上限" + itoa(limit) + "节",
			})
		}
		rows.Close()
	}
	// 班级课时不匹配
	rows, err = store.DB.Query(
		`SELECT g.name, c.name,
			(SELECT COALESCE(SUM(a.weekly_hours),0) FROM assignments a WHERE a.class_id=c.id),
			(SELECT COUNT(*) FROM timetable tt WHERE tt.class_id=c.id)
		 FROM classes c JOIN grades g ON c.grade_id=g.id WHERE c.enabled=1`)
	if err == nil {
		for rows.Next() {
			var gname, cname string
			var need, have int
			rows.Scan(&gname, &cname, &need, &have)
			if need > 0 && have < need {
				out = append(out, map[string]interface{}{
					"type": "hour_mismatch", "level": "warning",
					"message": "班级「" + gname + cname + "」课时不足：应排" + itoa(need) + "节，实际" + itoa(have) + "节",
				})
			}
		}
		rows.Close()
	}
	return out
}

func dayName2(d int) string {
	return map[int]string{1: "周一", 2: "周二", 3: "周三", 4: "周四", 5: "周五", 6: "周六", 7: "周日"}[d]
}

/* ---------- 单元格微调 ---------- */

func handleScheduleCell(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "schedule")
	if u == nil {
		return
	}
	var body struct {
		ClassID    int  `json:"class_id"`
		DayIndex   int  `json:"day_index"`
		PeriodIdx  int  `json:"period_index"`
		SubjectID  int  `json:"subject_id"`
		TeacherID  int  `json:"teacher_id"`
		Clear      bool `json:"clear"`
	}
	if err := bindJSON(r, &body); err != nil || body.ClassID <= 0 || body.DayIndex < 1 || body.PeriodIdx < 1 {
		fail(w, 400, "参数错误")
		return
	}
	// 班级有效性
	var classCnt int
	store.DB.QueryRow("SELECT COUNT(*) FROM classes WHERE id=?", body.ClassID).Scan(&classCnt)
	if classCnt == 0 {
		fail(w, 400, "班级不存在")
		return
	}
	store.DB.Exec("DELETE FROM timetable WHERE class_id=? AND day_index=? AND period_index=?",
		body.ClassID, body.DayIndex, body.PeriodIdx)
	if !body.Clear {
		if body.SubjectID <= 0 || body.TeacherID <= 0 {
			fail(w, 400, "请选择科目和教师")
			return
		}
		// 校验任课关系
		var cnt int
		store.DB.QueryRow("SELECT COUNT(*) FROM assignments WHERE class_id=? AND subject_id=? AND teacher_id=?", body.ClassID, body.SubjectID, body.TeacherID).Scan(&cnt)
		if cnt == 0 {
			// 允许配置过同科目不同教师的情况，校验该科目属于该班
			store.DB.QueryRow("SELECT COUNT(*) FROM assignments WHERE class_id=? AND subject_id=?", body.ClassID, body.SubjectID).Scan(&cnt)
			if cnt == 0 {
				fail(w, 400, "该班级未配置此科目")
				return
			}
		}
		store.DB.Exec("INSERT INTO timetable (class_id, day_index, period_index, subject_id, teacher_id, source) VALUES (?,?,?,?,?, 'manual')",
			body.ClassID, body.DayIndex, body.PeriodIdx, body.SubjectID, body.TeacherID)
	}
	AddLog(u, "update", "手动微调课表 班级#"+itoa(body.ClassID))
	okMsg(w, "已保存", nil)
}

/* ---------- 拖拽交换 ---------- */

type cellInfo struct {
	SubjectID int
	TeacherID int
}

func handleScheduleSwap(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "schedule")
	if u == nil {
		return
	}
	var body struct {
		ClassID int `json:"class_id"`
		From    struct {
			Day    int `json:"day"`
			Period int `json:"period"`
		} `json:"from"`
		To struct {
			Day    int `json:"day"`
			Period int `json:"period"`
		} `json:"to"`
	}
	if err := bindJSON(r, &body); err != nil || body.ClassID <= 0 {
		fail(w, 400, "参数错误")
		return
	}
	from := getCell(body.ClassID, body.From.Day, body.From.Period)
	to := getCell(body.ClassID, body.To.Day, body.To.Period)
	if from == nil {
		fail(w, 400, "源位置没有课程")
		return
	}
	// 冲突预检（排除本班，避免误判）
	if teacherBusy(body.To.Day, body.To.Period, from.TeacherID, body.ClassID) {
		fail(w, 400, "目标位置该教师已有其他班级课程，请选择其他位置")
		return
	}
	if to != nil && teacherBusy(body.From.Day, body.From.Period, to.TeacherID, body.ClassID) {
		fail(w, 400, "交换后会在源位置产生教师冲突，请选择其他位置")
		return
	}
	tx, err := store.DB.Begin()
	if err != nil {
		fail(w, 500, "事务失败")
		return
	}
	defer tx.Rollback()
	// 注意：事务已占用唯一连接，必须使用 tx.Exec 而非 store.DB.Exec，否则连接池死锁
	if _, err := tx.Exec("DELETE FROM timetable WHERE class_id=? AND day_index=? AND period_index=?",
		body.ClassID, body.From.Day, body.From.Period); err != nil {
		fail(w, 500, "保存失败")
		return
	}
	if to != nil {
		if _, err := tx.Exec("DELETE FROM timetable WHERE class_id=? AND day_index=? AND period_index=?",
			body.ClassID, body.To.Day, body.To.Period); err != nil {
			fail(w, 500, "保存失败")
			return
		}
		if _, err := tx.Exec("INSERT INTO timetable (class_id, day_index, period_index, subject_id, teacher_id, source) VALUES (?,?,?,?,?, 'manual')",
			body.ClassID, body.From.Day, body.From.Period, to.SubjectID, to.TeacherID); err != nil {
			fail(w, 500, "保存失败")
			return
		}
	}
	if _, err := tx.Exec("INSERT INTO timetable (class_id, day_index, period_index, subject_id, teacher_id, source) VALUES (?,?,?,?,?, 'manual')",
		body.ClassID, body.To.Day, body.To.Period, from.SubjectID, from.TeacherID); err != nil {
		fail(w, 500, "保存失败")
		return
	}
	if err := tx.Commit(); err != nil {
		fail(w, 500, "保存失败")
		return
	}
	AddLog(u, "update", "拖拽交换课表 班级#"+itoa(body.ClassID))
	okMsg(w, "交换成功", nil)
}

func getCell(classID, day, period int) *cellInfo {
	row := store.DB.QueryRow("SELECT subject_id, teacher_id FROM timetable WHERE class_id=? AND day_index=? AND period_index=?",
		classID, day, period)
	c := &cellInfo{}
	if err := row.Scan(&c.SubjectID, &c.TeacherID); err != nil {
		return nil
	}
	return c
}

// teacherBusy 检查某教师在指定时间是否已被其他班级占用
func teacherBusy(day, period, teacherID, exceptClassID int) bool {
	var cnt int
	store.DB.QueryRow("SELECT COUNT(*) FROM timetable WHERE day_index=? AND period_index=? AND teacher_id=? AND class_id<>?",
		day, period, teacherID, exceptClassID).Scan(&cnt)
	return cnt > 0
}

/* ---------- 课表查询 ---------- */

func handleClassTimetable(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "view")
	if u == nil {
		return
	}
	classID, err := strconv.Atoi(r.URL.Query().Get("class_id"))
	if err != nil || classID <= 0 {
		fail(w, 400, "参数错误")
		return
	}
	matrix := map[string]map[string]interface{}{}
	rows, err := store.DB.Query(
		`SELECT tt.day_index, tt.period_index, tt.subject_id, s.name, tt.teacher_id, t.name, s.subject_type, tt.source
		 FROM timetable tt
		 JOIN subjects s ON tt.subject_id=s.id
		 JOIN teachers t ON tt.teacher_id=t.id
		 WHERE tt.class_id=? ORDER BY tt.day_index, tt.period_index`, classID)
	if err != nil {
		fail(w, 500, "查询失败")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var d, p, sid, tid int
		var sname, tname, stype, source string
		if err := rows.Scan(&d, &p, &sid, &sname, &tid, &tname, &stype, &source); err == nil {
			dayKey := strconv.Itoa(d)
			if matrix[dayKey] == nil {
				matrix[dayKey] = map[string]interface{}{}
			}
			matrix[dayKey][strconv.Itoa(p)] = map[string]interface{}{
				"subject_id": sid, "subject_name": sname, "teacher_id": tid,
				"teacher_name": tname, "subject_type": stype, "source": source,
			}
		}
	}
	periods := getPeriods()
	ok(w, map[string]interface{}{"matrix": matrix, "periods": periods})
}

func handleTeacherTimetable(w http.ResponseWriter, r *http.Request) {
	u := requireLogin(w, r)
	if u == nil {
		return
	}
	teacherID := 0
	if u.Role == "teacher" {
		if u.TeacherID.Valid {
			teacherID = int(u.TeacherID.Int64)
		}
	} else {
		teacherID, _ = strconv.Atoi(r.URL.Query().Get("teacher_id"))
	}
	if teacherID <= 0 {
		fail(w, 400, "请选择教师")
		return
	}
	var teacherName string
	if err := store.DB.QueryRow("SELECT name FROM teachers WHERE id=?", teacherID).Scan(&teacherName); err != nil {
		fail(w, 404, "教师不存在")
		return
	}
	matrix := map[string]map[string]interface{}{}
	rows, err := store.DB.Query(
		`SELECT tt.day_index, tt.period_index, s.name, c.name, s.subject_type
		 FROM timetable tt
		 JOIN subjects s ON tt.subject_id=s.id
		 JOIN classes c ON tt.class_id=c.id
		 WHERE tt.teacher_id=? ORDER BY tt.day_index, tt.period_index`, teacherID)
	if err != nil {
		fail(w, 500, "查询失败")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var d, p int
		var sname, cname, stype string
		if err := rows.Scan(&d, &p, &sname, &cname, &stype); err == nil {
			dayKey := strconv.Itoa(d)
			if matrix[dayKey] == nil {
				matrix[dayKey] = map[string]interface{}{}
			}
			matrix[dayKey][strconv.Itoa(p)] = map[string]interface{}{
				"subject_name": sname, "class_name": cname, "subject_type": stype,
			}
		}
	}
	subjects := ""
	rows2, _ := store.DB.Query("SELECT s.name FROM teacher_subjects ts JOIN subjects s ON ts.subject_id=s.id WHERE ts.teacher_id=?", teacherID)
	for rows2.Next() {
		var n string
		rows2.Scan(&n)
		if subjects != "" {
			subjects += "、"
		}
		subjects += n
	}
	rows2.Close()
	ok(w, map[string]interface{}{
		"matrix": matrix,
		"periods": getPeriods(),
		"teacher": map[string]interface{}{
			"id": teacherID, "name": teacherName, "subject_names": subjects,
			"weekly_hour_limit": getTeacherLimit(teacherID),
		},
	})
}

func getPeriods() []map[string]interface{} {
	rows, err := store.DB.Query("SELECT period_index, start_time, end_time, period_type FROM periods ORDER BY period_index")
	if err != nil {
		return []map[string]interface{}{}
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var idx int
		var st, et, pt string
		if err := rows.Scan(&idx, &st, &et, &pt); err == nil {
			out = append(out, map[string]interface{}{
				"period_index": idx, "start_time": st, "end_time": et, "period_type": pt,
			})
		}
	}
	return out
}

func getTeacherLimit(teacherID int) int {
	var limit int
	store.DB.QueryRow("SELECT weekly_hour_limit FROM teachers WHERE id=?", teacherID).Scan(&limit)
	return limit
}

// ReqTeacherID 辅助：教师角色强制本人
func ReqTeacherID(u *auth.User, query string) int {
	if u.Role == "teacher" {
		if u.TeacherID.Valid {
			return int(u.TeacherID.Int64)
		}
		return 0
	}
	id, _ := strconv.Atoi(query)
	return id
}
