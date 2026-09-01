package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"school-scheduler/internal/store"
)

/* ==================== 年级 ==================== */

func handleGrades(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "view")
	if u == nil {
		return
	}
	if r.Method == "GET" {
		rows, err := store.DB.Query(
			"SELECT g.id, g.name, g.sort_order, g.enabled, (SELECT COUNT(*) FROM classes c WHERE c.grade_id=g.id) AS cnt FROM grades g ORDER BY g.sort_order")
		if err != nil {
			fail(w, 500, "查询失败")
			return
		}
		defer rows.Close()
		list := []map[string]interface{}{}
		for rows.Next() {
			var id, so, cnt int
			var name string
			var enabled bool
			if err := rows.Scan(&id, &name, &so, &enabled, &cnt); err == nil {
				list = append(list, map[string]interface{}{"id": id, "name": name, "sort_order": so, "enabled": enabled, "class_count": cnt})
			}
		}
		ok(w, list)
		return
	}
	// POST
	if requirePerm(w, r, "edit") == nil {
		return
	}
	var body struct {
		Name      string `json:"name"`
		SortOrder int    `json:"sort_order"`
	}
	if err := bindJSON(r, &body); err != nil || strings.TrimSpace(body.Name) == "" {
		fail(w, 400, "请填写年级名称")
		return
	}
	res, err := store.DB.Exec("INSERT INTO grades (name, sort_order) VALUES (?,?)", strings.TrimSpace(body.Name), body.SortOrder)
	if err != nil {
		fail(w, 400, "保存失败")
		return
	}
	id, _ := res.LastInsertId()
	AddLog(u, "create", "新增年级 "+body.Name)
	ok(w, map[string]interface{}{"id": id})
}

func handleGradeUpdate(w http.ResponseWriter, r *http.Request) {
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
		var cnt int
		store.DB.QueryRow("SELECT COUNT(*) FROM classes WHERE grade_id=?", id).Scan(&cnt)
		if cnt > 0 {
			fail(w, 400, "该年级下仍有班级，请先处理班级")
			return
		}
		store.DB.Exec("DELETE FROM grades WHERE id=?", id)
		AddLog(u, "delete", "删除年级 #"+itoa(id))
		okMsg(w, "删除成功", nil)
		return
	}
	var body struct {
		Name      string `json:"name"`
		SortOrder int    `json:"sort_order"`
		Enabled   *bool  `json:"enabled"`
	}
	if err := bindJSON(r, &body); err != nil {
		fail(w, 400, "参数错误")
		return
	}
	if body.Name != "" {
		store.DB.Exec("UPDATE grades SET name=? WHERE id=?", body.Name, id)
	}
	if body.SortOrder > 0 {
		store.DB.Exec("UPDATE grades SET sort_order=? WHERE id=?", body.SortOrder, id)
	}
	if body.Enabled != nil {
		store.DB.Exec("UPDATE grades SET enabled=? WHERE id=?", btoi(*body.Enabled), id)
	}
	AddLog(u, "update", "修改年级 #"+itoa(id))
	okMsg(w, "保存成功", nil)
}

/* ==================== 班级 ==================== */

func handleClasses(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "view")
	if u == nil {
		return
	}
	if r.Method == "GET" {
		rows, err := store.DB.Query(
			"SELECT c.id, c.grade_id, g.name, c.name, c.class_no, c.enabled FROM classes c JOIN grades g ON c.grade_id=g.id ORDER BY g.sort_order, c.class_no")
		if err != nil {
			fail(w, 500, "查询失败")
			return
		}
		defer rows.Close()
		list := []map[string]interface{}{}
		for rows.Next() {
			var id, gid, cn int
			var gname, cname string
			var enabled bool
			if err := rows.Scan(&id, &gid, &gname, &cname, &cn, &enabled); err == nil {
				list = append(list, map[string]interface{}{
					"id": id, "grade_id": gid, "grade_name": gname, "name": cname, "class_no": cn, "enabled": enabled,
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
		GradeID int      `json:"grade_id"`
		Names   []string `json:"names"`
	}
	if err := bindJSON(r, &body); err != nil || body.GradeID <= 0 || len(body.Names) == 0 {
		fail(w, 400, "请选择年级并填写班级名称")
		return
	}
	var gradeName string
	if err := store.DB.QueryRow("SELECT name FROM grades WHERE id=?", body.GradeID).Scan(&gradeName); err != nil {
		fail(w, 400, "年级不存在")
		return
	}
	// 计算起始 class_no
	var maxNo int
	store.DB.QueryRow("SELECT COALESCE(MAX(class_no),0) FROM classes WHERE grade_id=?", body.GradeID).Scan(&maxNo)
	created := []map[string]interface{}{}
	for _, raw := range body.Names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		maxNo++
		res, err := store.DB.Exec("INSERT INTO classes (grade_id, name, class_no) VALUES (?,?,?)", body.GradeID, name, maxNo)
		if err != nil {
			continue
		}
		id, _ := res.LastInsertId()
		created = append(created, map[string]interface{}{"id": id, "name": name})
	}
	AddLog(u, "create", "批量新增班级 "+gradeName+" "+itoa(len(created))+"个")
	ok(w, created)
}

func handleClassUpdate(w http.ResponseWriter, r *http.Request) {
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
		store.DB.Exec("DELETE FROM assignments WHERE class_id=?", id)
		store.DB.Exec("DELETE FROM timetable WHERE class_id=?", id)
		store.DB.Exec("DELETE FROM classes WHERE id=?", id)
		AddLog(u, "delete", "删除班级 #"+itoa(id))
		okMsg(w, "删除成功", nil)
		return
	}
	var body struct {
		GradeID int    `json:"grade_id"`
		Name    string `json:"name"`
		ClassNo int    `json:"class_no"`
		Enabled *bool  `json:"enabled"`
	}
	if err := bindJSON(r, &body); err != nil {
		fail(w, 400, "参数错误")
		return
	}
	if body.GradeID > 0 {
		store.DB.Exec("UPDATE classes SET grade_id=? WHERE id=?", body.GradeID, id)
	}
	if body.Name != "" {
		store.DB.Exec("UPDATE classes SET name=? WHERE id=?", body.Name, id)
	}
	if body.ClassNo > 0 {
		store.DB.Exec("UPDATE classes SET class_no=? WHERE id=?", body.ClassNo, id)
	}
	if body.Enabled != nil {
		store.DB.Exec("UPDATE classes SET enabled=? WHERE id=?", btoi(*body.Enabled), id)
	}
	AddLog(u, "update", "修改班级 #"+itoa(id))
	okMsg(w, "保存成功", nil)
}

/* ==================== 科目 ==================== */

func handleSubjects(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "view")
	if u == nil {
		return
	}
	if r.Method == "GET" {
		rows, err := store.DB.Query("SELECT id, name, subject_type, sort_order, is_default, enabled FROM subjects ORDER BY sort_order")
		if err != nil {
			fail(w, 500, "查询失败")
			return
		}
		defer rows.Close()
		list := []map[string]interface{}{}
		for rows.Next() {
			var id, so, isDefault int
			var name, st string
			var enabled bool
			if err := rows.Scan(&id, &name, &st, &so, &isDefault, &enabled); err == nil {
				tn := "副科"
				if st == "main" {
					tn = "主科"
				}
				list = append(list, map[string]interface{}{
					"id": id, "name": name, "subject_type": st, "subject_type_name": tn,
					"sort_order": so, "is_default": isDefault == 1, "enabled": enabled,
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
		Name        string `json:"name"`
		SubjectType string `json:"subject_type"`
		SortOrder   int    `json:"sort_order"`
	}
	if err := bindJSON(r, &body); err != nil || strings.TrimSpace(body.Name) == "" {
		fail(w, 400, "请填写科目名称")
		return
	}
	if body.SubjectType != "main" {
		body.SubjectType = "sub"
	}
	res, err := store.DB.Exec("INSERT INTO subjects (name, subject_type, sort_order) VALUES (?,?,?)",
		strings.TrimSpace(body.Name), body.SubjectType, body.SortOrder)
	if err != nil {
		fail(w, 400, "保存失败")
		return
	}
	id, _ := res.LastInsertId()
	AddLog(u, "create", "新增科目 "+body.Name)
	ok(w, map[string]interface{}{"id": id})
}

func handleSubjectUpdate(w http.ResponseWriter, r *http.Request) {
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
		store.DB.QueryRow("SELECT is_default FROM subjects WHERE id=?", id).Scan(&isDefault)
		if isDefault {
			fail(w, 400, "内置科目不允许删除，可禁用")
			return
		}
		store.DB.Exec("DELETE FROM subjects WHERE id=?", id)
		AddLog(u, "delete", "删除科目 #"+itoa(id))
		okMsg(w, "删除成功", nil)
		return
	}
	var body struct {
		Name        string `json:"name"`
		SubjectType string `json:"subject_type"`
		SortOrder   int    `json:"sort_order"`
		Enabled     *bool  `json:"enabled"`
	}
	if err := bindJSON(r, &body); err != nil {
		fail(w, 400, "参数错误")
		return
	}
	if body.Name != "" {
		store.DB.Exec("UPDATE subjects SET name=? WHERE id=?", body.Name, id)
	}
	if body.SubjectType == "main" || body.SubjectType == "sub" {
		store.DB.Exec("UPDATE subjects SET subject_type=? WHERE id=?", body.SubjectType, id)
	}
	if body.SortOrder > 0 {
		store.DB.Exec("UPDATE subjects SET sort_order=? WHERE id=?", body.SortOrder, id)
	}
	if body.Enabled != nil {
		store.DB.Exec("UPDATE subjects SET enabled=? WHERE id=?", btoi(*body.Enabled), id)
	}
	AddLog(u, "update", "修改科目 #"+itoa(id))
	okMsg(w, "保存成功", nil)
}

/* ==================== 教师 ==================== */

func handleTeachers(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "view")
	if u == nil {
		return
	}
	if r.Method == "GET" {
		// 注意：连接池为单连接，必须先关闭主查询 rows 再执行子查询，否则嵌套查询会死锁
		type teacherRow struct {
			id       int
			name     string
			phone    string
			isCT     bool
			limit    int
			allowEve bool
			remark   string
			enabled  bool
		}
		rows, err := store.DB.Query("SELECT id, name, phone, is_class_teacher, weekly_hour_limit, allow_evening, remark, enabled FROM teachers ORDER BY id")
		if err != nil {
			fail(w, 500, "查询失败")
			return
		}
		raw := []teacherRow{}
		for rows.Next() {
			var t teacherRow
			if err := rows.Scan(&t.id, &t.name, &t.phone, &t.isCT, &t.limit, &t.allowEve, &t.remark, &t.enabled); err == nil {
				raw = append(raw, t)
			}
		}
		rows.Close()
		list := []map[string]interface{}{}
		for _, t := range raw {
			subjects, subNames := teacherSubjects(t.id)
			postName := "普通教师"
			if t.isCT {
				postName = "班主任"
			}
			list = append(list, map[string]interface{}{
				"id": t.id, "name": t.name, "phone": t.phone, "is_class_teacher": t.isCT, "post_name": postName,
				"weekly_hour_limit": t.limit, "allow_evening": t.allowEve, "remark": t.remark, "enabled": t.enabled,
				"subjects": subjects, "subject_names": subNames,
			})
		}
		ok(w, list)
		return
	}
	if requirePerm(w, r, "edit") == nil {
		return
	}
	var body struct {
		Name            string `json:"name"`
		Phone           string `json:"phone"`
		IsClassTeacher  bool   `json:"is_class_teacher"`
		WeeklyHourLimit int    `json:"weekly_hour_limit"`
		AllowEvening    bool   `json:"allow_evening"`
		Remark          string `json:"remark"`
		Enabled         *bool  `json:"enabled"`
		SubjectIDs      []int  `json:"subject_ids"`
	}
	if err := bindJSON(r, &body); err != nil || strings.TrimSpace(body.Name) == "" {
		fail(w, 400, "请填写教师姓名")
		return
	}
	if body.WeeklyHourLimit <= 0 {
		body.WeeklyHourLimit = 20
	}
	enabled := 1 // 默认启用，前端不传 enabled 时新教师为可用状态
	if body.Enabled != nil {
		enabled = btoi(*body.Enabled)
	}
	res, err := store.DB.Exec("INSERT INTO teachers (name, phone, is_class_teacher, weekly_hour_limit, allow_evening, remark, enabled) VALUES (?,?,?,?,?,?,?)",
		strings.TrimSpace(body.Name), body.Phone, btoi(body.IsClassTeacher), body.WeeklyHourLimit, btoi(body.AllowEvening), body.Remark, enabled)
	if err != nil {
		fail(w, 400, "保存失败")
		return
	}
	id, _ := res.LastInsertId()
	saveTeacherSubjects(int(id), body.SubjectIDs)
	AddLog(u, "create", "新增教师 "+body.Name)
	ok(w, map[string]interface{}{"id": id})
}

func handleTeacherUpdate(w http.ResponseWriter, r *http.Request) {
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
		var cnt int
		store.DB.QueryRow("SELECT COUNT(*) FROM assignments WHERE teacher_id=?", id).Scan(&cnt)
		if cnt > 0 {
			fail(w, 400, "该教师仍有任课关系，请先删除")
			return
		}
		store.DB.Exec("DELETE FROM teacher_subjects WHERE teacher_id=?", id)
		store.DB.Exec("DELETE FROM teachers WHERE id=?", id)
		AddLog(u, "delete", "删除教师 #"+itoa(id))
		okMsg(w, "删除成功", nil)
		return
	}
	var body struct {
		Name            string `json:"name"`
		Phone           string `json:"phone"`
		IsClassTeacher  *bool  `json:"is_class_teacher"`
		WeeklyHourLimit int    `json:"weekly_hour_limit"`
		AllowEvening    *bool  `json:"allow_evening"`
		Remark          string `json:"remark"`
		Enabled         *bool  `json:"enabled"`
		SubjectIDs      []int  `json:"subject_ids"`
	}
	if err := bindJSON(r, &body); err != nil {
		fail(w, 400, "参数错误")
		return
	}
	if body.Name != "" {
		store.DB.Exec("UPDATE teachers SET name=? WHERE id=?", body.Name, id)
	}
	if body.Phone != "" {
		store.DB.Exec("UPDATE teachers SET phone=? WHERE id=?", body.Phone, id)
	}
	if body.IsClassTeacher != nil {
		store.DB.Exec("UPDATE teachers SET is_class_teacher=? WHERE id=?", btoi(*body.IsClassTeacher), id)
	}
	if body.WeeklyHourLimit > 0 {
		store.DB.Exec("UPDATE teachers SET weekly_hour_limit=? WHERE id=?", body.WeeklyHourLimit, id)
	}
	if body.AllowEvening != nil {
		store.DB.Exec("UPDATE teachers SET allow_evening=? WHERE id=?", btoi(*body.AllowEvening), id)
	}
	if body.Remark != "" {
		store.DB.Exec("UPDATE teachers SET remark=? WHERE id=?", body.Remark, id)
	}
	if body.Enabled != nil {
		store.DB.Exec("UPDATE teachers SET enabled=? WHERE id=?", btoi(*body.Enabled), id)
	}
	if body.SubjectIDs != nil {
		saveTeacherSubjects(id, body.SubjectIDs)
	}
	AddLog(u, "update", "修改教师 #"+itoa(id))
	okMsg(w, "保存成功", nil)
}

// teacherSubjects 查询教师任教科目
func teacherSubjects(teacherID int) ([]map[string]interface{}, string) {
	rows, err := store.DB.Query(
		"SELECT s.id, s.name FROM teacher_subjects ts JOIN subjects s ON ts.subject_id=s.id WHERE ts.teacher_id=? ORDER BY s.sort_order",
		teacherID)
	if err != nil {
		return []map[string]interface{}{}, ""
	}
	defer rows.Close()
	list := []map[string]interface{}{}
	names := []string{}
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err == nil {
			list = append(list, map[string]interface{}{"id": id, "name": name})
			names = append(names, name)
		}
	}
	return list, strings.Join(names, "、")
}

func saveTeacherSubjects(teacherID int, ids []int) {
	store.DB.Exec("DELETE FROM teacher_subjects WHERE teacher_id=?", teacherID)
	for _, sid := range ids {
		store.DB.Exec("INSERT OR IGNORE INTO teacher_subjects (teacher_id, subject_id) VALUES (?,?)", teacherID, sid)
	}
}

/* ==================== 任课关系 ==================== */

func handleAssignments(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "view")
	if u == nil {
		return
	}
	if r.Method == "GET" {
		rows, err := store.DB.Query(
			`SELECT a.id, a.class_id, g.name, c.name, a.subject_id, s.name, a.teacher_id, t.name, a.weekly_hours, a.prefer_morning
			 FROM assignments a
			 JOIN classes c ON a.class_id=c.id
			 JOIN grades g ON c.grade_id=g.id
			 JOIN subjects s ON a.subject_id=s.id
			 JOIN teachers t ON a.teacher_id=t.id
			 ORDER BY g.sort_order, c.class_no, s.sort_order`)
		if err != nil {
			fail(w, 500, "查询失败")
			return
		}
		defer rows.Close()
		list := []map[string]interface{}{}
		for rows.Next() {
			var id, cid, sid, tid, wh int
			var gname, cname, sname, tname string
			var pm bool
			if err := rows.Scan(&id, &cid, &gname, &cname, &sid, &sname, &tid, &tname, &wh, &pm); err == nil {
				list = append(list, map[string]interface{}{
					"id": id, "class_id": cid, "grade_name": gname, "class_name": cname,
					"subject_id": sid, "subject_name": sname, "teacher_id": tid, "teacher_name": tname,
					"weekly_hours": wh, "prefer_morning": pm,
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
		ClassID       int  `json:"class_id"`
		SubjectID     int  `json:"subject_id"`
		TeacherID     int  `json:"teacher_id"`
		WeeklyHours   int  `json:"weekly_hours"`
		PreferMorning bool `json:"prefer_morning"`
	}
	if err := bindJSON(r, &body); err != nil || body.ClassID <= 0 || body.SubjectID <= 0 || body.TeacherID <= 0 {
		fail(w, 400, "请完整填写班级/科目/教师")
		return
	}
	if body.WeeklyHours <= 0 {
		body.WeeklyHours = 1
	}
	// 校验教师可教该科目
	var subjCnt int
	store.DB.QueryRow("SELECT COUNT(*) FROM teacher_subjects WHERE teacher_id=? AND subject_id=?", body.TeacherID, body.SubjectID).Scan(&subjCnt)
	if subjCnt == 0 {
		fail(w, 400, "该教师未绑定此任教科目")
		return
	}
	// 校验同班同科目不重复
	var dup int
	store.DB.QueryRow("SELECT COUNT(*) FROM assignments WHERE class_id=? AND subject_id=?", body.ClassID, body.SubjectID).Scan(&dup)
	if dup > 0 {
		fail(w, 400, "该班级此科目已配置任课关系")
		return
	}
	res, err := store.DB.Exec("INSERT INTO assignments (class_id, subject_id, teacher_id, weekly_hours, prefer_morning) VALUES (?,?,?,?,?)",
		body.ClassID, body.SubjectID, body.TeacherID, body.WeeklyHours, btoi(body.PreferMorning))
	if err != nil {
		fail(w, 500, "保存失败")
		return
	}
	id, _ := res.LastInsertId()
	AddLog(u, "create", "新增任课关系 #"+itoa(int(id)))
	ok(w, map[string]interface{}{"id": id})
}

func handleAssignmentUpdate(w http.ResponseWriter, r *http.Request) {
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
		store.DB.Exec("DELETE FROM assignments WHERE id=?", id)
		AddLog(u, "delete", "删除任课关系 #"+itoa(id))
		okMsg(w, "删除成功", nil)
		return
	}
	var body struct {
		ClassID       int  `json:"class_id"`
		SubjectID     int  `json:"subject_id"`
		TeacherID     int  `json:"teacher_id"`
		WeeklyHours   int  `json:"weekly_hours"`
		PreferMorning bool `json:"prefer_morning"`
	}
	if err := bindJSON(r, &body); err != nil {
		fail(w, 400, "参数错误")
		return
	}
	if body.ClassID > 0 {
		store.DB.Exec("UPDATE assignments SET class_id=? WHERE id=?", body.ClassID, id)
	}
	if body.SubjectID > 0 {
		store.DB.Exec("UPDATE assignments SET subject_id=? WHERE id=?", body.SubjectID, id)
	}
	if body.TeacherID > 0 {
		store.DB.Exec("UPDATE assignments SET teacher_id=? WHERE id=?", body.TeacherID, id)
	}
	if body.WeeklyHours > 0 {
		store.DB.Exec("UPDATE assignments SET weekly_hours=? WHERE id=?", body.WeeklyHours, id)
	}
	store.DB.Exec("UPDATE assignments SET prefer_morning=? WHERE id=?", btoi(body.PreferMorning), id)
	AddLog(u, "update", "修改任课关系 #"+itoa(id))
	okMsg(w, "保存成功", nil)
}

func handleAssignmentsClear(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "edit")
	if u == nil {
		return
	}
	var body struct {
		ClassID int `json:"class_id"`
	}
	if err := bindJSON(r, &body); err != nil {
		fail(w, 400, "参数错误")
		return
	}
	if body.ClassID > 0 {
		store.DB.Exec("DELETE FROM assignments WHERE class_id=?", body.ClassID)
	} else {
		store.DB.Exec("DELETE FROM assignments")
	}
	AddLog(u, "delete", "清空任课关系")
	okMsg(w, "已清空", nil)
}

/* ==================== Excel 导入 ==================== */

func handleImportTemplate(w http.ResponseWriter, r *http.Request) {
	if requirePerm(w, r, "view") == nil {
		return
	}
	kind := r.PathValue("kind")
	data, name, err := buildTemplate(kind)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Write(data)
}

func handleImport(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "edit")
	if u == nil {
		return
	}
	kind := r.PathValue("kind")
	r.ParseMultipartForm(32 << 20)
	file, _, err := r.FormFile("file")
	if err != nil {
		fail(w, 400, "请选择文件")
		return
	}
	defer file.Close()
	res, err := importExcel(kind, file)
	if err != nil {
		fail(w, 400, "导入失败："+err.Error())
		return
	}
	AddLog(u, "import", "Excel导入"+kind+" 成功"+itoa(res.Success)+"条")
	ok(w, map[string]interface{}{
		"success_count": res.Success,
		"fail_count":    len(res.FailMsgs),
		"fail_msgs":     res.FailMsgs,
	})
}

// parseJSONParam 解析 JSON 参数
func parseJSONParam(s string) map[string]interface{} {
	m := map[string]interface{}{}
	json.Unmarshal([]byte(s), &m)
	return m
}
