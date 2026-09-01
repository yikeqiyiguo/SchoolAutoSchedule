package api

import (
	"database/sql"
	"net/http"

	"school-scheduler/internal/store"
)

// handleStatsOverview 仪表盘统计
func handleStatsOverview(w http.ResponseWriter, r *http.Request) {
	u := requireLogin(w, r)
	if u == nil {
		return
	}
	var grades, classes, teachers, subjects, assignments, scheduled, needTotal int
	store.DB.QueryRow("SELECT COUNT(*) FROM grades").Scan(&grades)
	store.DB.QueryRow("SELECT COUNT(*) FROM classes WHERE enabled=1").Scan(&classes)
	store.DB.QueryRow("SELECT COUNT(*) FROM teachers WHERE enabled=1").Scan(&teachers)
	store.DB.QueryRow("SELECT COUNT(*) FROM subjects WHERE enabled=1").Scan(&subjects)
	store.DB.QueryRow("SELECT COUNT(*) FROM assignments").Scan(&assignments)
	store.DB.QueryRow("SELECT COUNT(*) FROM timetable").Scan(&scheduled)
	store.DB.QueryRow("SELECT COALESCE(SUM(weekly_hours),0) FROM assignments").Scan(&needTotal)
	var schoolName string
	store.DB.QueryRow("SELECT school_name FROM system_config WHERE id=1").Scan(&schoolName)
	percent := 0
	if needTotal > 0 {
		percent = scheduled * 100 / needTotal
	}
	ok(w, map[string]interface{}{
		"grades": grades, "classes": classes, "teachers": teachers,
		"subjects": subjects, "assignments": assignments,
		"need_total": needTotal, "scheduled": scheduled, "schedule_percent": percent,
		"school_name": schoolName,
	})
}

// handleStatsTeacher 教师课时统计
func handleStatsTeacher(w http.ResponseWriter, r *http.Request) {
	u := requireLogin(w, r)
	if u == nil {
		return
	}
	query := `SELECT t.id, t.name, t.weekly_hour_limit,
		(SELECT COALESCE(SUM(a.weekly_hours),0) FROM assignments a WHERE a.teacher_id=t.id),
		(SELECT COUNT(*) FROM timetable tt WHERE tt.teacher_id=t.id)
		FROM teachers t WHERE t.enabled=1 ORDER BY t.id`
	args := []interface{}{}
	if u.Role == "teacher" {
		query = `SELECT t.id, t.name, t.weekly_hour_limit,
			(SELECT COALESCE(SUM(a.weekly_hours),0) FROM assignments a WHERE a.teacher_id=t.id),
			(SELECT COUNT(*) FROM timetable tt WHERE tt.teacher_id=t.id)
			FROM teachers t WHERE t.enabled=1 AND t.id=? ORDER BY t.id`
		if u.TeacherID.Valid {
			args = append(args, int(u.TeacherID.Int64))
		} else {
			args = append(args, -1)
		}
	}
	rows, err := store.DB.Query(query, args...)
	if err != nil {
		fail(w, 500, "查询失败")
		return
	}
	// 注意：连接池为单连接，必须先关闭主查询 rows 再执行子查询，否则嵌套查询会死锁
	type statRow struct {
		id, limit, need, have int
		name                  string
	}
	raw := []statRow{}
	for rows.Next() {
		var t statRow
		if err := rows.Scan(&t.id, &t.name, &t.limit, &t.need, &t.have); err != nil {
			continue
		}
		raw = append(raw, t)
	}
	rows.Close()
	list := []map[string]interface{}{}
	for _, t := range raw {
		srows, _ := store.DB.Query("SELECT s.name FROM teacher_subjects ts JOIN subjects s ON ts.subject_id=s.id WHERE ts.teacher_id=? ORDER BY s.sort_order", t.id)
		names := []string{}
		for srows.Next() {
			var n string
			srows.Scan(&n)
			names = append(names, n)
		}
		srows.Close()
		subs := joinNames(names)
		remain := t.need - t.have
		if remain < 0 {
			remain = 0
		}
		percent := 0
		if t.need > 0 {
			percent = t.have * 100 / t.need
		}
		list = append(list, map[string]interface{}{
			"teacher_id": t.id, "teacher_name": t.name, "subjects": subs,
			"need": t.need, "have": t.have, "remain": remain, "percent": percent, "limit": t.limit,
		})
	}
	ok(w, list)
}

// handleStatsClass 班级课时统计
func handleStatsClass(w http.ResponseWriter, r *http.Request) {
	u := requireLogin(w, r)
	if u == nil {
		return
	}
	query := `SELECT c.id, g.name, c.name,
		(SELECT COALESCE(SUM(a.weekly_hours),0) FROM assignments a WHERE a.class_id=c.id),
		(SELECT COUNT(*) FROM timetable tt WHERE tt.class_id=c.id)
		FROM classes c JOIN grades g ON c.grade_id=g.id WHERE c.enabled=1 ORDER BY g.sort_order, c.class_no`
	var rows *sql.Rows
	var err error
	if u.Role == "teacher" {
		myClasses := map[int]bool{}
		if u.TeacherID.Valid {
			tid := int(u.TeacherID.Int64)
			crows, _ := store.DB.Query("SELECT DISTINCT class_id FROM assignments WHERE teacher_id=?", tid)
			for crows.Next() {
				var cid int
				crows.Scan(&cid)
				myClasses[cid] = true
			}
			crows.Close()
		}
		if len(myClasses) == 0 {
			ok(w, []map[string]interface{}{})
			return
		}
		query = `SELECT c.id, g.name, c.name,
			(SELECT COALESCE(SUM(a.weekly_hours),0) FROM assignments a WHERE a.class_id=c.id),
			(SELECT COUNT(*) FROM timetable tt WHERE tt.class_id=c.id)
			FROM classes c JOIN grades g ON c.grade_id=g.id WHERE c.enabled=1 AND c.id IN (SELECT class_id FROM assignments WHERE teacher_id=?)
			ORDER BY g.sort_order, c.class_no`
		rows, err = store.DB.Query(query, int(u.TeacherID.Int64))
	} else {
		rows, err = store.DB.Query(query)
	}
	if err != nil {
		fail(w, 500, "查询失败")
		return
	}
	defer rows.Close()
	list := []map[string]interface{}{}
	for rows.Next() {
		var cid, need, have int
		var gname, cname string
		if err := rows.Scan(&cid, &gname, &cname, &need, &have); err != nil {
			continue
		}
		remain := need - have
		if remain < 0 {
			remain = 0
		}
		percent := 0
		if need > 0 {
			percent = have * 100 / need
		}
		list = append(list, map[string]interface{}{
			"class_id": cid, "class_name": cname, "grade_name": gname,
			"need": need, "have": have, "remain": remain, "percent": percent,
		})
	}
	ok(w, list)
}

func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += "、"
		}
		out += n
	}
	return out
}


