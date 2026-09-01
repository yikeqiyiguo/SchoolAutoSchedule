package api

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"

	"school-scheduler/internal/store"
)

// ImportResult 导入结果
type ImportResult struct {
	Success  int
	FailMsgs []string
}

func buildTemplate(kind string) ([]byte, string, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"
	var headers []string
	switch kind {
	case "classes":
		headers = []string{"年级", "班级名称"}
	case "teachers":
		headers = []string{"姓名", "手机号", "岗位(班主任/普通)", "周课时上限", "允许晚自习(是/否)", "任教科目(用/分隔)"}
	case "assignments":
		headers = []string{"年级", "班级", "科目", "教师", "每周课时", "优先上午(是/否)"}
	default:
		return nil, "", errors.New("不支持的模板类型")
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	// 示例行
	switch kind {
	case "classes":
		f.SetCellValue(sheet, "A2", "一年级")
		f.SetCellValue(sheet, "B2", "1班")
	case "teachers":
		f.SetCellValue(sheet, "A2", "张三")
		f.SetCellValue(sheet, "B2", "13800000000")
		f.SetCellValue(sheet, "C2", "班主任")
		f.SetCellValue(sheet, "D2", 20)
		f.SetCellValue(sheet, "E2", "是")
		f.SetCellValue(sheet, "F2", "语文/数学")
	case "assignments":
		f.SetCellValue(sheet, "A2", "一年级")
		f.SetCellValue(sheet, "B2", "1班")
		f.SetCellValue(sheet, "C2", "语文")
		f.SetCellValue(sheet, "D2", "张三")
		f.SetCellValue(sheet, "E2", 5)
		f.SetCellValue(sheet, "F2", "是")
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "导入模板-" + kind + ".xlsx", nil
}

func importExcel(kind string, r io.Reader) (*ImportResult, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, errors.New("文件解析失败，请上传 .xlsx/.xls 文件")
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("Excel 中没有工作表")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, errors.New("读取工作表失败")
	}
	res := &ImportResult{}
	if len(rows) < 2 {
		return res, nil
	}
	gradeCache := map[string]int{}
	classCache := map[string]int{}
	subjectCache := map[string]int{}
	teacherCache := map[string]int{}

	loadCache := func(table string, cache map[string]int) {
		rows2, _ := store.DB.Query("SELECT id, name FROM " + table)
		defer rows2.Close()
		for rows2.Next() {
			var id int
			var name string
			rows2.Scan(&id, &name)
			cache[strings.TrimSpace(name)] = id
		}
	}
	loadCache("grades", gradeCache)
	loadCache("classes", classCache)
	loadCache("subjects", subjectCache)
	loadCache("teachers", teacherCache)

	// classes 缓存需要 grade 前缀区分：gradeName+className
	fullClassCache := map[string]int{}
	{
		rows2, _ := store.DB.Query("SELECT c.id, g.name, c.name FROM classes c JOIN grades g ON c.grade_id=g.id")
		defer rows2.Close()
		for rows2.Next() {
			var id int
			var gn, cn string
			rows2.Scan(&id, &gn, &cn)
			fullClassCache[gn+"|"+cn] = id
		}
	}
	// teachers 任教科目缓存
	teacherSubjectsCache := map[int][]int{}
	{
		rows2, _ := store.DB.Query("SELECT teacher_id, subject_id FROM teacher_subjects")
		defer rows2.Close()
		for rows2.Next() {
			var tid, sid int
			rows2.Scan(&tid, &sid)
			teacherSubjectsCache[tid] = append(teacherSubjectsCache[tid], sid)
		}
	}

	toBool := func(v string) bool {
		v = strings.TrimSpace(v)
		return v == "是" || v == "y" || v == "Y" || v == "true" || v == "1"
	}
	toInt := func(v string, def int) int {
		n := 0
		if _, err := fmt.Sscanf(strings.TrimSpace(v), "%d", &n); err != nil {
			return def
		}
		return n
	}

	for i := 1; i < len(rows); i++ { // 跳过表头
		row := rows[i]
		line := i + 1
		fail := func(msg string) {
			res.FailMsgs = append(res.FailMsgs, fmt.Sprintf("第%d行：%s", line, msg))
		}
		cell := func(idx int) string {
			if idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}
		switch kind {
		case "classes":
			gname, cname := cell(0), cell(1)
			if gname == "" || cname == "" {
				fail("年级/班级名称不能为空")
				continue
			}
			gid, ok := gradeCache[gname]
			if !ok {
				// 自动创建年级
				var maxSo int
				store.DB.QueryRow("SELECT COALESCE(MAX(sort_order),0) FROM grades").Scan(&maxSo)
				res2, err := store.DB.Exec("INSERT INTO grades (name, sort_order) VALUES (?,?)", gname, maxSo+1)
				if err != nil {
					fail("年级创建失败")
					continue
				}
				id, _ := res2.LastInsertId()
				gid = int(id)
				gradeCache[gname] = gid
			}
			if _, dup := fullClassCache[gname+"|"+cname]; dup {
				fail("班级已存在：" + cname)
				continue
			}
			var maxNo int
			store.DB.QueryRow("SELECT COALESCE(MAX(class_no),0) FROM classes WHERE grade_id=?", gid).Scan(&maxNo)
			res2, err := store.DB.Exec("INSERT INTO classes (grade_id, name, class_no) VALUES (?,?,?)", gid, cname, maxNo+1)
			if err != nil {
				fail("班级创建失败")
				continue
			}
			id, _ := res2.LastInsertId()
			fullClassCache[gname+"|"+cname] = int(id)
			res.Success++
		case "teachers":
			name, phone := cell(0), cell(1)
			post := cell(2)
			limit := toInt(cell(3), 20)
			allowEve := toBool(cell(4))
			subjectsStr := cell(5)
			if name == "" {
				fail("姓名不能为空")
				continue
			}
			if tid, dup := teacherCache[name]; dup {
				// 更新
				store.DB.Exec("UPDATE teachers SET phone=?, weekly_hour_limit=?, allow_evening=? WHERE id=?", phone, limit, btoi(allowEve), tid)
				if subjectsStr != "" {
					ids := resolveSubjectIDs(subjectsStr, subjectCache)
					saveTeacherSubjects(tid, ids)
				}
				res.Success++
				continue
			}
			res2, err := store.DB.Exec("INSERT INTO teachers (name, phone, is_class_teacher, weekly_hour_limit, allow_evening) VALUES (?,?,?,?,?)",
				name, phone, btoi(post == "班主任"), limit, btoi(allowEve))
			if err != nil {
				fail("教师创建失败")
				continue
			}
			id, _ := res2.LastInsertId()
			tid := int(id)
			teacherCache[name] = tid
			if subjectsStr != "" {
				saveTeacherSubjects(tid, resolveSubjectIDs(subjectsStr, subjectCache))
			}
			res.Success++
		case "assignments":
			gname, cname, sname, tname := cell(0), cell(1), cell(2), cell(3)
			hours := toInt(cell(4), 1)
			preferMorning := toBool(cell(5))
			if gname == "" || cname == "" || sname == "" || tname == "" {
				fail("年级/班级/科目/教师不能为空")
				continue
			}
			key := gname + "|" + cname
			cid, okC := fullClassCache[key]
			if !okC {
				fail(fmt.Sprintf("班级不存在：%s %s（请先导入班级）", gname, cname))
				continue
			}
			sid, okS := subjectCache[sname]
			if !okS {
				fail("科目不存在：" + sname)
				continue
			}
			tid, okT := teacherCache[tname]
			if !okT {
				fail("教师不存在：" + tname)
				continue
			}
			// 教师可教校验
			canTeach := false
			for _, sid2 := range teacherSubjectsCache[tid] {
				if sid2 == sid {
					canTeach = true
					break
				}
			}
			if !canTeach {
				fail(fmt.Sprintf("教师「%s」未绑定科目「%s」", tname, sname))
				continue
			}
			var dup int
			store.DB.QueryRow("SELECT COUNT(*) FROM assignments WHERE class_id=? AND subject_id=?", cid, sid).Scan(&dup)
			if dup > 0 {
				fail(fmt.Sprintf("班级「%s」科目「%s」已配置", cname, sname))
				continue
			}
			if _, err := store.DB.Exec("INSERT INTO assignments (class_id, subject_id, teacher_id, weekly_hours, prefer_morning) VALUES (?,?,?,?,?)",
				cid, sid, tid, hours, btoi(preferMorning)); err != nil {
				fail("任课关系创建失败")
				continue
			}
			res.Success++
		}
	}
	if len(res.FailMsgs) > 0 {
		res.FailMsgs = append([]string{fmt.Sprintf("成功 %d 条，失败 %d 条", res.Success, len(res.FailMsgs))}, res.FailMsgs...)
	}
	return res, nil
}

func resolveSubjectIDs(str string, cache map[string]int) []int {
	ids := []int{}
	for _, name := range strings.FieldsFunc(str, func(r rune) bool { return r == '/' || r == '、' || r == ',' || r == '，' }) {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if id, ok := cache[name]; ok {
			ids = append(ids, id)
		} else {
			// 自动创建科目
			var maxSo int
			store.DB.QueryRow("SELECT COALESCE(MAX(sort_order),0) FROM subjects").Scan(&maxSo)
			res2, err := store.DB.Exec("INSERT INTO subjects (name, subject_type, sort_order) VALUES (?,?,?)", name, "sub", maxSo+1)
			if err != nil {
				continue
			}
			id, _ := res2.LastInsertId()
			cache[name] = int(id)
			ids = append(ids, int(id))
		}
	}
	return ids
}
