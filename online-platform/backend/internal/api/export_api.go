package api

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/phpdave11/gofpdf"
	"github.com/xuri/excelize/v2"

	"school-scheduler/internal/store"
)

/* ---------- 通用数据读取 ---------- */

type ttCell struct {
	SubjectID   int
	SubjectName string
	SubjectType string
	TeacherName string
	ClassName   string
	Source      string
}

func loadMatrix(classID int) map[int]map[int]*ttCell {
	m := map[int]map[int]*ttCell{}
	rows, err := store.DB.Query(
		`SELECT tt.day_index, tt.period_index, tt.subject_id, s.name, tt.teacher_id, t.name, s.subject_type, tt.source
		 FROM timetable tt JOIN subjects s ON tt.subject_id=s.id JOIN teachers t ON tt.teacher_id=t.id
		 WHERE tt.class_id=?`, classID)
	if err != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var d, p, sid, tid int
		var sname, tname, stype, source string
		if err := rows.Scan(&d, &p, &sid, &sname, &tid, &tname, &stype, &source); err == nil {
			if m[d] == nil {
				m[d] = map[int]*ttCell{}
			}
			m[d][p] = &ttCell{SubjectID: sid, SubjectName: sname, SubjectType: stype, TeacherName: tname, Source: source}
		}
	}
	return m
}

func loadTeacherMatrix(teacherID int) map[int]map[int]*ttCell {
	m := map[int]map[int]*ttCell{}
	rows, err := store.DB.Query(
		`SELECT tt.day_index, tt.period_index, s.name, s.subject_type, c.name
		 FROM timetable tt JOIN subjects s ON tt.subject_id=s.id JOIN classes c ON tt.class_id=c.id
		 WHERE tt.teacher_id=?`, teacherID)
	if err != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var d, p int
		var sname, stype, cname string
		if err := rows.Scan(&d, &p, &sname, &stype, &cname); err == nil {
			if m[d] == nil {
				m[d] = map[int]*ttCell{}
			}
			m[d][p] = &ttCell{SubjectName: sname, SubjectType: stype, ClassName: cname}
		}
	}
	return m
}

func getPeriodRows() []map[string]interface{} { return getPeriods() }

// schoolName 学校名称
func schoolName() string {
	var s string
	store.DB.QueryRow("SELECT school_name FROM system_config WHERE id=1").Scan(&s)
	return s
}

// schoolDays 上课日
func schoolDays() []int {
	var s string
	store.DB.QueryRow("SELECT school_days FROM system_config WHERE id=1").Scan(&s)
	out := []int{}
	for _, c := range s {
		if c >= '1' && c <= '7' {
			out = append(out, int(c-'0'))
		}
	}
	if len(out) == 0 {
		out = []int{1, 2, 3, 4, 5}
	}
	return out
}

// setDownloadHeader 设置下载响应头
func setDownloadHeader(w http.ResponseWriter, contentType, filename string) {
	w.Header().Set("Content-Type", contentType)
	dis := fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		filename, url.PathEscape(filename))
	w.Header().Set("Content-Disposition", dis)
}

/* ---------- Excel 导出 ---------- */

const excelCT = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

var dayNameList = []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}

// fillClassSheet 填充一个班级课表 sheet
func fillClassSheet(f *excelize.File, sheet, title string, matrix map[int]map[int]*ttCell) {
	f.NewSheet(sheet)
	f.SetCellValue(sheet, "A1", title)
	f.MergeCell(sheet, "A1", "H1")
	// 表头
	head := []string{"节次", "时间"}
	days := schoolDays()
	for _, d := range days {
		head = append(head, dayNameList[d-1])
	}
	for i, h := range head {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue(sheet, cell, h)
	}
	periods := getPeriodRows()
	row := 3
	for _, p := range periods {
		idx := p["period_index"].(int)
		st := p["start_time"].(string)
		et := p["end_time"].(string)
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("第%d节", idx))
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), st+"\n"+et)
		for ci, d := range days {
			col, _ := excelize.CoordinatesToCellName(ci+3, row)
			if cell := matrix[d][idx]; cell != nil {
				f.SetCellValue(sheet, col, cell.SubjectName+"\n"+cell.TeacherName)
			} else {
				f.SetCellValue(sheet, col, "")
			}
		}
		row++
	}
	styleHeader, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"3B6EF6"}}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}})
	lastCol, _ := excelize.CoordinatesToCellName(len(head), 2)
	f.SetCellStyle(sheet, "A2", lastCol, styleHeader)
	f.SetRowHeight(sheet, 1, 24)
	f.SetRowHeight(sheet, 2, 20)
	for i := 0; i < row-2; i++ {
		f.SetRowHeight(sheet, 3+i, 34)
	}
	f.SetColWidth(sheet, "A", "A", 10)
	f.SetColWidth(sheet, "B", "B", 14)
	f.SetColWidth(sheet, "C", "H", 18)
}

func handleExportAllClasses(w http.ResponseWriter, r *http.Request) {
	if requirePerm(w, r, "export") == nil {
		return
	}
	f := excelize.NewFile()
	defer f.Close()
	f.DeleteSheet("Sheet1")
	// 注意：连接池为单连接，必须先关闭主查询 rows 再执行子查询，否则嵌套查询会死锁
	type classRow struct {
		cid         int
		gname, cname string
	}
	rows, _ := store.DB.Query(
		"SELECT c.id, g.name, c.name FROM classes c JOIN grades g ON c.grade_id=g.id WHERE c.enabled=1 ORDER BY g.sort_order, c.class_no")
	classes := []classRow{}
	for rows.Next() {
		var cr classRow
		if err := rows.Scan(&cr.cid, &cr.gname, &cr.cname); err == nil {
			classes = append(classes, cr)
		}
	}
	rows.Close()
	for _, cr := range classes {
		sheet := sanitizeSheet(cr.gname + cr.cname)
		m := loadMatrix(cr.cid)
		fillClassSheet(f, sheet, fmt.Sprintf("%s %s%s 课程表", schoolName(), cr.gname, cr.cname), m)
	}
	f.SetActiveSheet(0)
	buf, err := f.WriteToBuffer()
	if err != nil {
		fail(w, 500, "导出失败")
		return
	}
	AddLog(CurrentUser(r), "export", "导出全校班级课表")
	setDownloadHeader(w, excelCT, "全校班级课表.xlsx")
	w.Write(buf.Bytes())
}

func handleExportClassExcel(w http.ResponseWriter, r *http.Request) {
	if requirePerm(w, r, "export") == nil {
		return
	}
	id, okID := pathID(r, "id")
	if !okID {
		fail(w, 400, "参数错误")
		return
	}
	var gname, cname string
	if err := store.DB.QueryRow("SELECT g.name, c.name FROM classes c JOIN grades g ON c.grade_id=g.id WHERE c.id=?", id).Scan(&gname, &cname); err != nil {
		fail(w, 404, "班级不存在")
		return
	}
	f := excelize.NewFile()
	defer f.Close()
	m := loadMatrix(id)
	fillClassSheet(f, "课表", fmt.Sprintf("%s %s%s 课程表", schoolName(), gname, cname), m)
	buf, err := f.WriteToBuffer()
	if err != nil {
		fail(w, 500, "导出失败")
		return
	}
	AddLog(CurrentUser(r), "export", "导出班级课表 "+gname+cname)
	setDownloadHeader(w, excelCT, gname+cname+"课程表.xlsx")
	w.Write(buf.Bytes())
}

func handleExportAllTeachers(w http.ResponseWriter, r *http.Request) {
	if requirePerm(w, r, "export") == nil {
		return
	}
	f := excelize.NewFile()
	defer f.Close()
	f.DeleteSheet("Sheet1")
	// 注意：连接池为单连接，必须先关闭主查询 rows 再执行子查询，否则嵌套查询会死锁
	type teacherRow struct {
		tid   int
		tname string
	}
	rows, _ := store.DB.Query("SELECT id, name FROM teachers WHERE enabled=1 ORDER BY id")
	teachers := []teacherRow{}
	for rows.Next() {
		var tr teacherRow
		if err := rows.Scan(&tr.tid, &tr.tname); err == nil {
			teachers = append(teachers, tr)
		}
	}
	rows.Close()
	for _, tr := range teachers {
		sheet := sanitizeSheet(tr.tname)
		m := loadTeacherMatrix(tr.tid)
		fillTeacherSheet(f, sheet, fmt.Sprintf("%s %s 课程表", schoolName(), tr.tname), m)
	}
	f.SetActiveSheet(0)
	buf, err := f.WriteToBuffer()
	if err != nil {
		fail(w, 500, "导出失败")
		return
	}
	AddLog(CurrentUser(r), "export", "导出全校教师课表")
	setDownloadHeader(w, excelCT, "全校教师课表.xlsx")
	w.Write(buf.Bytes())
}

func fillTeacherSheet(f *excelize.File, sheet, title string, matrix map[int]map[int]*ttCell) {
	f.NewSheet(sheet)
	f.SetCellValue(sheet, "A1", title)
	f.MergeCell(sheet, "A1", "H1")
	head := []string{"节次", "时间"}
	days := schoolDays()
	for _, d := range days {
		head = append(head, dayNameList[d-1])
	}
	for i, h := range head {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue(sheet, cell, h)
	}
	periods := getPeriodRows()
	row := 3
	for _, p := range periods {
		idx := p["period_index"].(int)
		st := p["start_time"].(string)
		et := p["end_time"].(string)
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("第%d节", idx))
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), st+"\n"+et)
		for ci, d := range days {
			col, _ := excelize.CoordinatesToCellName(ci+3, row)
			if cell := matrix[d][idx]; cell != nil {
				f.SetCellValue(sheet, col, cell.SubjectName+"\n"+cell.ClassName)
			}
		}
		row++
	}
	styleHeader, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"7C3AED"}}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}})
	lastCol, _ := excelize.CoordinatesToCellName(len(head), 2)
	f.SetCellStyle(sheet, "A2", lastCol, styleHeader)
	f.SetRowHeight(sheet, 1, 24)
	f.SetRowHeight(sheet, 2, 20)
	for i := 0; i < row-2; i++ {
		f.SetRowHeight(sheet, 3+i, 34)
	}
	f.SetColWidth(sheet, "A", "A", 10)
	f.SetColWidth(sheet, "B", "B", 14)
	f.SetColWidth(sheet, "C", "H", 18)
}

func handleExportTeacherExcel(w http.ResponseWriter, r *http.Request) {
	u := requireLogin(w, r)
	if u == nil {
		return
	}
	id, okID := pathID(r, "id")
	if !okID {
		fail(w, 400, "参数错误")
		return
	}
	if u.Role == "teacher" {
		if !u.TeacherID.Valid || int(u.TeacherID.Int64) != id {
			forbidden(w)
			return
		}
	}
	var tname string
	if err := store.DB.QueryRow("SELECT name FROM teachers WHERE id=?", id).Scan(&tname); err != nil {
		fail(w, 404, "教师不存在")
		return
	}
	f := excelize.NewFile()
	defer f.Close()
	m := loadTeacherMatrix(id)
	fillTeacherSheet(f, "课表", fmt.Sprintf("%s %s 课程表", schoolName(), tname), m)
	buf, err := f.WriteToBuffer()
	if err != nil {
		fail(w, 500, "导出失败")
		return
	}
	AddLog(u, "export", "导出教师课表 "+tname)
	setDownloadHeader(w, excelCT, tname+"课程表.xlsx")
	w.Write(buf.Bytes())
}

func handleExportStats(w http.ResponseWriter, r *http.Request) {
	if requirePerm(w, r, "export") == nil {
		return
	}
	f := excelize.NewFile()
	defer f.Close()
	// 教师统计
	f.SetSheetName("Sheet1", "教师课时统计")
	f.SetCellValue(f.GetSheetName(0), "A1", schoolName()+" 教师课时统计")
	f.MergeCell(f.GetSheetName(0), "A1", "G1")
	heads := []string{"教师", "任教科目", "应排课时", "已排课时", "剩余课时", "使用率", "上限"}
	for i, h := range heads {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue(f.GetSheetName(0), cell, h)
	}
	// 注意：连接池为单连接，必须先关闭主查询 rows 再执行子查询，否则嵌套查询会死锁
	type statRow struct {
		name        string
		limit, need, have int
	}
	rows, _ := store.DB.Query(
		`SELECT t.name, t.weekly_hour_limit,
			(SELECT COALESCE(SUM(a.weekly_hours),0) FROM assignments a WHERE a.teacher_id=t.id),
			(SELECT COUNT(*) FROM timetable tt WHERE tt.teacher_id=t.id)
		 FROM teachers t WHERE t.enabled=1 ORDER BY t.id`)
	stats := []statRow{}
	for rows.Next() {
		var s statRow
		if err := rows.Scan(&s.name, &s.limit, &s.need, &s.have); err == nil {
			stats = append(stats, s)
		}
	}
	rows.Close()
	row := 3
	for _, s := range stats {
		percent := 0
		if s.need > 0 {
			percent = s.have * 100 / s.need
		}
		subs := ""
		srows, _ := store.DB.Query("SELECT s.name FROM teacher_subjects ts JOIN subjects s ON ts.subject_id=s.id WHERE ts.teacher_id IN (SELECT id FROM teachers WHERE name=?)", s.name)
		names := []string{}
		for srows.Next() {
			var n string
			srows.Scan(&n)
			names = append(names, n)
		}
		srows.Close()
		subs = strings.Join(names, "、")
		vals := []interface{}{s.name, subs, s.need, s.have, s.need - s.have, fmt.Sprintf("%d%%", percent), s.limit}
		for i, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(i+1, row)
			f.SetCellValue(f.GetSheetName(0), cell, v)
		}
		row++
	}
	// 班级统计
	f.NewSheet("班级课时统计")
	f.SetCellValue("班级课时统计", "A1", schoolName()+" 班级课时统计")
	f.MergeCell("班级课时统计", "A1", "F1")
	heads2 := []string{"年级", "班级", "应排课时", "已排课时", "剩余课时", "完成度"}
	for i, h := range heads2 {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue("班级课时统计", cell, h)
	}
	rows2, _ := store.DB.Query(
		`SELECT g.name, c.name,
			(SELECT COALESCE(SUM(a.weekly_hours),0) FROM assignments a WHERE a.class_id=c.id),
			(SELECT COUNT(*) FROM timetable tt WHERE tt.class_id=c.id)
		 FROM classes c JOIN grades g ON c.grade_id=g.id WHERE c.enabled=1 ORDER BY g.sort_order, c.class_no`)
	defer rows2.Close()
	row = 3
	for rows2.Next() {
		var gname, cname string
		var need, have int
		rows2.Scan(&gname, &cname, &need, &have)
		percent := 0
		if need > 0 {
			percent = have * 100 / need
		}
		vals := []interface{}{gname, cname, need, have, need - have, fmt.Sprintf("%d%%", percent)}
		for i, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(i+1, row)
			f.SetCellValue("班级课时统计", cell, v)
		}
		row++
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		fail(w, 500, "导出失败")
		return
	}
	AddLog(CurrentUser(r), "export", "导出课时统计报表")
	setDownloadHeader(w, excelCT, "课时统计报表.xlsx")
	w.Write(buf.Bytes())
}

func sanitizeSheet(name string) string {
	bad := []string{"\\", "/", "?", "*", "[", "]", ":"}
	for _, b := range bad {
		name = strings.ReplaceAll(name, b, "_")
	}
	if len(name) > 28 {
		name = name[:28]
	}
	if name == "" {
		name = "Sheet"
	}
	return name
}

/* ---------- PDF 导出 ---------- */

var pdfFontPath string

// findChineseFont 查找系统中文字体（ttf）
func findChineseFont() string {
	if pdfFontPath != "" {
		return pdfFontPath
	}
	candidates := []string{
		`C:\Windows\Fonts\simhei.ttf`,
		`C:\Windows\Fonts\simfang.ttf`,
		`C:\Windows\Fonts\Deng.ttf`,
		`C:\Windows\Fonts\msyh.ttf`,
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			pdfFontPath = p
			return p
		}
	}
	return ""
}

// buildTimetablePDF 生成课表 PDF
func buildTimetablePDF(title string, matrix map[int]map[int]*ttCell, teacherMode bool) ([]byte, error) {
	fontPath := findChineseFont()
	if fontPath == "" {
		return nil, fmt.Errorf("未找到系统中文字体，无法生成PDF，请使用Excel导出")
	}
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddUTF8FontFromBytes("ch", "", readFileBytes(fontPath))
	pdf.AddPage()

	// 标题
	pdf.SetFont("ch", "", 16)
	pdf.SetTextColor(40, 40, 40)
	pdf.CellFormat(0, 12, title, "", 0, "C", false, 0, "")
	pdf.Ln(16)

	days := schoolDays()
	periods := getPeriodRows()
	colW := (275.0 - 22) / float64(len(days)+1)
	rowH := 14.0

	// 表头
	pdf.SetFont("ch", "", 9)
	pdf.SetFillColor(59, 110, 246)
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(10, rowH, "节次", "1", 0, "C", true, 0, "")
	pdf.CellFormat(12, rowH, "时间", "1", 0, "C", true, 0, "")
	for _, d := range days {
		pdf.CellFormat(colW, rowH, dayNameList[d-1], "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// 内容行
	pdf.SetTextColor(30, 30, 30)
	for _, p := range periods {
		idx := p["period_index"].(int)
		st := p["start_time"].(string)
		et := p["end_time"].(string)
		pdf.CellFormat(10, rowH, "第"+itoa(idx)+"节", "1", 0, "C", false, 0, "")
		pdf.CellFormat(12, rowH, st+"\n"+et, "1", 0, "C", false, 0, "")
		for _, d := range days {
			if cell := matrix[d][idx]; cell != nil {
				sub := cell.SubjectName
				by := cell.TeacherName
				if teacherMode {
					by = cell.ClassName
				}
				text := sub + "\n" + by
				if cell.SubjectType == "main" {
					pdf.SetFillColor(232, 239, 255)
				} else {
					pdf.SetFillColor(236, 253, 245)
				}
				pdf.CellFormat(colW, rowH, text, "1", 0, "C", true, 0, "")
			} else {
				pdf.CellFormat(colW, rowH, "", "1", 0, "C", false, 0, "")
			}
		}
		pdf.Ln(-1)
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func readFileBytes(path string) []byte {
	b, _ := os.ReadFile(path)
	return b
}

func handleExportClassPDF(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "export")
	if u == nil {
		return
	}
	id, okID := pathID(r, "id")
	if !okID {
		fail(w, 400, "参数错误")
		return
	}
	var gname, cname string
	if err := store.DB.QueryRow("SELECT g.name, c.name FROM classes c JOIN grades g ON c.grade_id=g.id WHERE c.id=?", id).Scan(&gname, &cname); err != nil {
		fail(w, 404, "班级不存在")
		return
	}
	buf, err := buildTimetablePDF(fmt.Sprintf("%s %s%s 课程表", schoolName(), gname, cname), loadMatrix(id), false)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	AddLog(u, "export", "导出班级课表PDF "+gname+cname)
	setDownloadHeader(w, "application/pdf", gname+cname+"课程表.pdf")
	w.Write(buf)
}

func handleExportTeacherPDF(w http.ResponseWriter, r *http.Request) {
	u := requireLogin(w, r)
	if u == nil {
		return
	}
	id, okID := pathID(r, "id")
	if !okID {
		fail(w, 400, "参数错误")
		return
	}
	if u.Role == "teacher" {
		if !u.TeacherID.Valid || int(u.TeacherID.Int64) != id {
			forbidden(w)
			return
		}
	}
	var tname string
	if err := store.DB.QueryRow("SELECT name FROM teachers WHERE id=?", id).Scan(&tname); err != nil {
		fail(w, 404, "教师不存在")
		return
	}
	buf, err := buildTimetablePDF(fmt.Sprintf("%s %s 课程表", schoolName(), tname), loadTeacherMatrix(id), true)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	AddLog(u, "export", "导出教师课表PDF "+tname)
	setDownloadHeader(w, "application/pdf", tname+"课程表.pdf")
	w.Write(buf)
}
