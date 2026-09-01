package main

// TestE2E 端到端集成测试：启动 build/scheduler-dev.exe（headless 模式），
// 通过 HTTP 客户端验证 登录 → 基础数据 → 排课 → 课表 → 统计 → 导出 → 备份 → 权限 等核心链路。
// 使用独立临时数据目录，不影响正式数据。
// 运行：go test -v -run TestE2E -timeout 10m

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

/* ---------- HTTP 客户端封装 ---------- */

type apiClient struct {
	base   string
	cookie string
	client *http.Client
}

func (c *apiClient) httpClient() *http.Client {
	if c.client != nil {
		return c.client
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (c *apiClient) req(t *testing.T, method, path string, body interface{}) (int, []byte, http.Header) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cookie != "" {
		req.Header.Set("Cookie", "sas_token="+c.cookie)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		t.Fatalf("%s %s 请求失败: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw, resp.Header
}

func (c *apiClient) json(t *testing.T, method, path string, body interface{}) (int, map[string]interface{}) {
	t.Helper()
	code, raw, _ := c.req(t, method, path, body)
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("%s %s 响应不是 JSON: %s", method, path, string(raw))
	}
	return code, m
}

func (c *apiClient) expectOK(t *testing.T, method, path string, body interface{}) map[string]interface{} {
	t.Helper()
	code, m := c.json(t, method, path, body)
	if code != 200 || !boolOf(m["success"]) {
		t.Fatalf("%s %s 期望成功(200) 实际 %d: %s", method, path, code, dumpJSON(m))
	}
	return m
}

func (c *apiClient) expectStatus(t *testing.T, want int, method, path string, body interface{}) map[string]interface{} {
	t.Helper()
	code, m := c.json(t, method, path, body)
	if code != want {
		t.Fatalf("%s %s 期望状态 %d 实际 %d: %s", method, path, want, code, dumpJSON(m))
	}
	return m
}

func (c *apiClient) login(t *testing.T, username, password string) {
	t.Helper()
	code, _, hdr := c.req(t, "POST", "/api/auth/login", map[string]interface{}{"username": username, "password": password})
	if code != 200 {
		t.Fatalf("登录失败 %s: %d", username, code)
	}
	for _, ck := range hdr.Values("Set-Cookie") {
		if strings.HasPrefix(ck, "sas_token=") {
			c.cookie = strings.SplitN(ck, ";", 2)[0][len("sas_token="):]
		}
	}
	if c.cookie == "" {
		t.Fatalf("登录成功但未返回会话 Cookie")
	}
}

func (c *apiClient) upload(t *testing.T, path, field, filename string, data []byte) map[string]interface{} {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(data)
	w.Close()
	req, err := http.NewRequest("POST", c.base+path, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if c.cookie != "" {
		req.Header.Set("Cookie", "sas_token="+c.cookie)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("上传响应不是 JSON: %s", string(raw))
	}
	return m
}

/* ---------- 工具 ---------- */

func boolOf(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func num(v interface{}) int {
	f, _ := v.(float64)
	return int(f)
}

func atoiStr(s string) int {
	n := 0
	fmt.Sscanf(s, "%d", &n)
	return n
}

func dumpJSON(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func dataOf(t *testing.T, m map[string]interface{}) interface{} {
	t.Helper()
	d, ok := m["data"]
	if !ok {
		t.Fatalf("响应缺少 data: %s", dumpJSON(m))
	}
	return d
}

func dataMap(t *testing.T, m map[string]interface{}) map[string]interface{} {
	t.Helper()
	dm, ok := dataOf(t, m).(map[string]interface{})
	if !ok {
		t.Fatalf("data 不是对象: %s", dumpJSON(m))
	}
	return dm
}

func dataList(t *testing.T, m map[string]interface{}) []interface{} {
	t.Helper()
	dl, ok := dataOf(t, m).([]interface{})
	if !ok {
		t.Fatalf("data 不是数组: %s", dumpJSON(m))
	}
	return dl
}

func findID(t *testing.T, list []interface{}, name string) int {
	t.Helper()
	for _, it := range list {
		mm, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		if s, ok := mm["name"].(string); ok && s == name {
			return num(mm["id"])
		}
	}
	t.Fatalf("列表中未找到 %s", name)
	return 0
}

func freePort() int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 18011
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func waitReady(t *testing.T, base string) {
	t.Helper()
	for i := 0; i < 60; i++ {
		resp, err := http.Get(base + "/")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("服务未在 30 秒内就绪")
}

/* ---------- 主测试 ---------- */

func TestE2E(t *testing.T) {
	exe := filepath.Join("build", "scheduler.exe")
	if abs, err := filepath.Abs(exe); err == nil {
		exe = abs
	}
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("未找到 %s，请先执行 go build -o build/scheduler-dev.exe .", exe)
	}
	port := freePort()
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	dataDir := t.TempDir()

	outLog, _ := os.CreateTemp("", "sas-e2e-*.log")
	defer outLog.Close()

	cmd := exec.Command(exe)
	// 过滤父进程已有的 SAS_* 变量，确保注入值唯一生效（避免 Windows 下重复键读到旧值）
	extra := []string{"SAS_HEADLESS=1", "SAS_PORT=" + fmt.Sprint(port), "SAS_DATA_DIR=" + dataDir}
	env := make([]string, 0, len(os.Environ())+len(extra))
	for _, kv := range os.Environ() {
		k := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k = kv[:i]
		}
		if strings.HasPrefix(k, "SAS_") {
			continue
		}
		env = append(env, kv)
	}
	cmd.Env = append(env, extra...)
	cmd.Stdout = outLog
	cmd.Stderr = outLog
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()
	t.Cleanup(func() {
		if t.Failed() {
			b, _ := os.ReadFile(outLog.Name())
			t.Logf("===== 服务日志 =====")
			t.Logf("%s", string(b))
		}
	})

	waitReady(t, base)

	c := &apiClient{base: base}

	// 1. 未登录访问受保护接口 → 401
	anon := &apiClient{base: base}
	anon.expectStatus(t, 401, "GET", "/api/base/subjects", nil)

	// 2. 登录
	c.login(t, "admin", "admin123")

	// 3. 静态资源
	code, raw, hdr := c.req(t, "GET", "/", nil)
	if code != 200 || len(raw) < 100 {
		t.Fatalf("首页加载失败 %d", code)
	}
	if ct := hdr.Get("Content-Type"); !strings.Contains(ct, "text/html") && !strings.Contains(ct, "html") {
		t.Fatalf("首页 Content-Type 异常: %s", ct)
	}
	if code, raw, _ = c.req(t, "GET", "/js/app.js", nil); code != 200 || len(raw) < 1000 {
		t.Fatalf("/js/app.js 加载失败 code=%d len=%d", code, len(raw))
	}
	if code, _, _ = c.req(t, "GET", "/css/app.css", nil); code != 200 {
		t.Fatalf("/css/app.css 加载失败 %d", code)
	}

	// 4. 当前用户信息
	m := c.expectOK(t, "GET", "/api/auth/me", nil)
	if dm := dataMap(t, m); dm["username"] != "admin" || dm["role"] != "super" {
		t.Fatalf("me 数据异常: %s", dumpJSON(dm))
	}

	// 5. 系统配置与元数据
	m = c.expectOK(t, "GET", "/api/config/meta", nil)
	if dm := dataMap(t, m); dm["rule_types"] == nil || dm["action_names"] == nil {
		t.Fatalf("meta 数据异常")
	}
	m = c.expectOK(t, "GET", "/api/config/system", nil)
	if dm := dataMap(t, m); len(dm["school_days"].([]interface{})) != 5 {
		t.Fatalf("school_days 异常: %s", dumpJSON(dm))
	}
	m = c.expectOK(t, "GET", "/api/config/periods", nil)
	periodList := dataList(t, m)
	if len(periodList) != 10 {
		t.Fatalf("默认时段应为 10 个，实际 %d", len(periodList))
	}
	c.expectOK(t, "PUT", "/api/config/periods", map[string]interface{}{"periods": periodList})

	// 6. 科目（内置 18 个）
	m = c.expectOK(t, "GET", "/api/base/subjects", nil)
	subjects := dataList(t, m)
	if len(subjects) < 18 {
		t.Fatalf("内置科目应不少于 18 个，实际 %d", len(subjects))
	}
	sid := map[string]int{}
	for _, name := range []string{"语文", "数学", "英语", "物理", "化学", "历史", "地理", "政治", "体育", "音乐", "美术", "信息科技"} {
		sid[name] = findID(t, subjects, name)
	}

	// 7. 软约束规则
	m = c.expectOK(t, "GET", "/api/rules", nil)
	ruleList := dataList(t, m)
	if len(ruleList) != 7 {
		t.Fatalf("默认规则应为 7 条，实际 %d", len(ruleList))
	}
	c.expectOK(t, "PUT", fmt.Sprintf("/api/rules/%d", num(ruleList[0].(map[string]interface{})["id"])), map[string]interface{}{"priority": 88})

	// 8. 创建年级 / 班级（批量）/ 删除
	m = c.expectOK(t, "POST", "/api/base/grades", map[string]interface{}{"name": "一年级", "sort_order": 1})
	gradeID := num(dataMap(t, m)["id"])
	m = c.expectOK(t, "POST", "/api/base/classes", map[string]interface{}{"grade_id": gradeID, "names": []string{"1班", "2班"}})
	created := dataList(t, m)
	if len(created) != 2 {
		t.Fatalf("批量创建班级应返回 2 个，实际 %d", len(created))
	}
	c.expectOK(t, "DELETE", fmt.Sprintf("/api/base/classes/%d", num(created[1].(map[string]interface{})["id"])), nil)
	m = c.expectOK(t, "GET", "/api/base/classes", nil)
	classID := findID(t, dataList(t, m), "1班")

	// 9. 创建教师（各绑定任教科目）
	type teacherSpec struct {
		name    string
		subject string
		isCT    bool
	}
	teacherSpecs := []teacherSpec{
		{"王老师", "语文", true},
		{"李老师", "数学", false},
		{"张老师", "英语", false},
		{"刘老师", "物理", false},
		{"陈老师", "化学", false},
		{"赵老师", "历史", false},
		{"周老师", "地理", false},
		{"吴老师", "政治", false},
		{"孙老师", "体育", false},
		{"郑老师", "音乐", false},
		{"冯老师", "美术", false},
		{"韩老师", "信息科技", false},
	}
	tid := map[string]int{}
	for _, ts := range teacherSpecs {
		m = c.expectOK(t, "POST", "/api/base/teachers", map[string]interface{}{
			"name": ts.name, "is_class_teacher": ts.isCT, "weekly_hour_limit": 20,
			"allow_evening": false, "subject_ids": []int{sid[ts.subject]},
		})
		tid[ts.name] = num(dataMap(t, m)["id"])
	}
	m = c.expectOK(t, "GET", "/api/base/teachers", nil)
	if len(dataList(t, m)) != 12 {
		t.Fatalf("教师数量应为 12")
	}

	// 10. 创建任课关系（1 班共 32 节）
	type assignSpec struct {
		subject string
		teacher string
		hours   int
		pm      bool
	}
	assigns := []assignSpec{
		{"语文", "王老师", 5, true},
		{"数学", "李老师", 5, true},
		{"英语", "张老师", 4, false},
		{"物理", "刘老师", 3, false},
		{"化学", "陈老师", 3, false},
		{"历史", "赵老师", 2, false},
		{"地理", "周老师", 2, false},
		{"政治", "吴老师", 2, false},
		{"体育", "孙老师", 2, false},
		{"音乐", "郑老师", 1, false},
		{"美术", "冯老师", 1, false},
		{"信息科技", "韩老师", 2, false},
	}
	totalHours := 0
	for _, as := range assigns {
		c.expectOK(t, "POST", "/api/base/assignments", map[string]interface{}{
			"class_id": classID, "subject_id": sid[as.subject], "teacher_id": tid[as.teacher],
			"weekly_hours": as.hours, "prefer_morning": as.pm,
		})
		totalHours += as.hours
	}
	if totalHours != 32 {
		t.Fatalf("测试数据课时总和应为 32，实际 %d", totalHours)
	}
	// 非法任课：孙老师未绑定数学 → 400
	m = c.expectStatus(t, 400, "POST", "/api/base/assignments", map[string]interface{}{
		"class_id": classID, "subject_id": sid["数学"], "teacher_id": tid["孙老师"], "weekly_hours": 1,
	})
	if msg, _ := m["message"].(string); !strings.Contains(msg, "未绑定") {
		t.Fatalf("非法任课应提示未绑定科目: %s", dumpJSON(m))
	}
	m = c.expectOK(t, "GET", "/api/base/assignments", nil)
	if len(dataList(t, m)) != 12 {
		t.Fatalf("任课关系应为 12 条")
	}

	// 11. 一键排课：应全排且零冲突
	m = c.expectOK(t, "POST", "/api/schedule/run", map[string]interface{}{"keep_manual": false})
	if num(m["placed"]) != totalHours {
		t.Fatalf("排课 placed=%d 应为 %d: %s", num(m["placed"]), totalHours, dumpJSON(m))
	}
	if cc := num(m["conflict_count"]); cc != 0 {
		t.Fatalf("排课应零冲突，实际 %d: %s", cc, dumpJSON(m))
	}

	// 12. 冲突检测应为空
	m = c.expectOK(t, "GET", "/api/schedule/conflicts", nil)
	if list := dataList(t, m); len(list) != 0 {
		t.Fatalf("冲突检测应无冲突: %s", dumpJSON(list))
	}

	// 13. 排课总览
	m = c.expectOK(t, "GET", "/api/schedule/overview", nil)
	{
		dm := dataMap(t, m)
		if num(dm["total_classes"]) != 1 {
			t.Fatalf("总班级数应为 1")
		}
		cls := dm["classes"].([]interface{})
		first := cls[0].(map[string]interface{})
		if num(first["have"]) != totalHours || num(first["percent"]) != 100 {
			t.Fatalf("班级排课进度应为 100%%: %s", dumpJSON(first))
		}
	}

	// 14. 统计
	m = c.expectOK(t, "GET", "/api/stats/overview", nil)
	{
		dm := dataMap(t, m)
		if num(dm["scheduled"]) != totalHours || num(dm["need_total"]) != totalHours {
			t.Fatalf("统计课时不匹配: %s", dumpJSON(dm))
		}
	}
	m = c.expectOK(t, "GET", "/api/stats/teacher", nil)
	for _, it := range dataList(t, m) {
		mm := it.(map[string]interface{})
		if num(mm["have"]) != num(mm["need"]) {
			t.Fatalf("教师 %v 课时未排满: %s", mm["teacher_name"], dumpJSON(mm))
		}
	}
	m = c.expectOK(t, "GET", "/api/stats/class", nil)
	if list := dataList(t, m); len(list) != 1 || num(list[0].(map[string]interface{})["have"]) != totalHours {
		t.Fatalf("班级统计异常: %s", dumpJSON(list))
	}

	// 15. 课表查询
	getMatrix := func() map[string]interface{} {
		m := c.expectOK(t, "GET", fmt.Sprintf("/api/schedule/timetable/class?class_id=%d", classID), nil)
		mx, ok := dataMap(t, m)["matrix"].(map[string]interface{})
		if !ok {
			t.Fatalf("课表 matrix 异常")
		}
		return mx
	}
	countCells := func(mx map[string]interface{}) int {
		n := 0
		for _, days := range mx {
			n += len(days.(map[string]interface{}))
		}
		return n
	}
	mx := getMatrix()
	if n := countCells(mx); n != totalHours {
		t.Fatalf("班级课表应含 %d 节课，实际 %d", totalHours, n)
	}
	m = c.expectOK(t, "GET", fmt.Sprintf("/api/schedule/timetable/teacher?teacher_id=%d", tid["王老师"]), nil)
	{
		dm := dataMap(t, m)
		if n := countCells(dm["matrix"].(map[string]interface{})); n != 5 {
			t.Fatalf("王老师课表应为 5 节，实际 %d", n)
		}
		if tm, ok := dm["teacher"].(map[string]interface{}); !ok || tm["name"] != "王老师" {
			t.Fatalf("教师课表信息异常: %s", dumpJSON(dm))
		}
	}

	// 16. 单元格微调（设 / 清）
	c.expectOK(t, "POST", "/api/schedule/cell", map[string]interface{}{
		"class_id": classID, "day_index": 1, "period_index": 1,
		"subject_id": sid["数学"], "teacher_id": tid["李老师"], "clear": false,
	})
	{
		mx := getMatrix()
		cell, ok := mx["1"].(map[string]interface{})["1"].(map[string]interface{})
		if !ok || cell["subject_name"] != "数学" {
			t.Fatalf("单元格微调未生效: %s", dumpJSON(mx))
		}
	}
	c.expectOK(t, "POST", "/api/schedule/cell", map[string]interface{}{
		"class_id": classID, "day_index": 1, "period_index": 1, "clear": true,
	})

	// 17. 拖拽交换
	cellOf := func(mx map[string]interface{}, d, p int) map[string]interface{} {
		days, _ := mx[fmt.Sprint(d)].(map[string]interface{})
		cell, _ := days[fmt.Sprint(p)].(map[string]interface{})
		return cell
	}
	var cells [][2]int
	mx = getMatrix()
	for dk, days := range mx {
		d := atoiStr(dk)
		for pk := range days.(map[string]interface{}) {
			cells = append(cells, [2]int{d, atoiStr(pk)})
			if len(cells) == 2 {
				break
			}
		}
		if len(cells) == 2 {
			break
		}
	}
	if len(cells) != 2 {
		t.Fatalf("课表格子不足，无法测试交换")
	}
	before := map[string]string{}
	for _, dp := range cells {
		before[fmt.Sprintf("%d-%d", dp[0], dp[1])] = cellOf(mx, dp[0], dp[1])["subject_name"].(string)
	}
	c.expectOK(t, "POST", "/api/schedule/swap", map[string]interface{}{
		"class_id": classID,
		"from":     map[string]interface{}{"day": cells[0][0], "period": cells[0][1]},
		"to":       map[string]interface{}{"day": cells[1][0], "period": cells[1][1]},
	})
	mx = getMatrix()
	gotF := cellOf(mx, cells[0][0], cells[0][1])["subject_name"]
	gotT := cellOf(mx, cells[1][0], cells[1][1])["subject_name"]
	keyFrom := fmt.Sprintf("%d-%d", cells[0][0], cells[0][1])
	keyTo := fmt.Sprintf("%d-%d", cells[1][0], cells[1][1])
	if gotF != before[keyTo] || gotT != before[keyFrom] {
		t.Fatalf("拖拽交换未生效: %s", dumpJSON(mx))
	}
	// 重新排课恢复状态
	m = c.expectOK(t, "POST", "/api/schedule/run", map[string]interface{}{"keep_manual": false})
	if num(m["conflict_count"]) != 0 {
		t.Fatalf("重新排课后应零冲突: %s", dumpJSON(m))
	}

	// 18. Excel / PDF 导出
	checkXLSX := func(path string) {
		code, raw, _ := c.req(t, "GET", path, nil)
		if code != 200 || len(raw) < 1000 || !(raw[0] == 'P' && raw[1] == 'K') {
			t.Fatalf("导出 %s 失败 code=%d len=%d", path, code, len(raw))
		}
	}
	checkPDF := func(path string) {
		code, raw, _ := c.req(t, "GET", path, nil)
		if code != 200 || len(raw) < 100 || !strings.HasPrefix(string(raw), "%PDF") {
			t.Fatalf("导出 %s 失败 code=%d len=%d", path, code, len(raw))
		}
	}
	checkXLSX("/api/export/all-classes/excel")
	checkXLSX(fmt.Sprintf("/api/export/class/%d/excel", classID))
	checkXLSX("/api/export/stats/excel")
	checkPDF(fmt.Sprintf("/api/export/class/%d/pdf", classID))
	checkPDF(fmt.Sprintf("/api/export/teacher/%d/pdf", tid["王老师"]))

	// 19. 导入模板下载 + 教师导入
	code, raw, _ = c.req(t, "GET", "/api/base/import/template/teachers", nil)
	if code != 200 || len(raw) < 2 || !(raw[0] == 'P' && raw[1] == 'K') {
		t.Fatalf("导入模板下载失败 code=%d", code)
	}
	{
		f := excelize.NewFile()
		headers := []string{"姓名", "手机号", "岗位(班主任/普通)", "周课时上限", "允许晚自习(是/否)", "任教科目(用/分隔)"}
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue("Sheet1", cell, h)
		}
		f.SetCellValue("Sheet1", "A2", "导入教师甲")
		f.SetCellValue("Sheet1", "B2", "13900000000")
		f.SetCellValue("Sheet1", "C2", "普通")
		f.SetCellValue("Sheet1", "D2", 20)
		f.SetCellValue("Sheet1", "E2", "是")
		f.SetCellValue("Sheet1", "F2", "语文/数学")
		buf, err := f.WriteToBuffer()
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		m = c.upload(t, "/api/base/import/teachers", "file", "teachers.xlsx", buf.Bytes())
		if dm := dataMap(t, m); num(dm["success_count"]) < 1 {
			t.Fatalf("教师导入应成功: %s", dumpJSON(dm))
		}
	}

	// 20. 备份：创建 / 列表 / 下载
	c.expectOK(t, "POST", "/api/backup/create", nil)
	m = c.expectOK(t, "GET", "/api/backup", nil)
	bkList := dataList(t, m)
	if len(bkList) == 0 {
		t.Fatalf("备份列表为空")
	}
	code, raw, _ = c.req(t, "GET", fmt.Sprintf("/api/backup/download/%d", num(bkList[0].(map[string]interface{})["id"])), nil)
	if code != 200 || !strings.HasPrefix(string(raw), "SQLite format 3") {
		t.Fatalf("备份下载失败 code=%d", code)
	}

	// 21. 用户管理
	m = c.expectOK(t, "POST", "/api/auth/users", map[string]interface{}{
		"username": "op1", "real_name": "操作员一", "role": "operator", "password": "123456",
	})
	opID := num(dataMap(t, m)["id"])
	c.expectOK(t, "POST", fmt.Sprintf("/api/auth/reset-password/%d", opID), map[string]interface{}{"password": "654321"})
	c.expectOK(t, "PUT", fmt.Sprintf("/api/auth/users/%d", opID), map[string]interface{}{"real_name": "操作员改名", "role": "teacher"})
	c.expectOK(t, "DELETE", fmt.Sprintf("/api/auth/users/%d", opID), nil)
	m = c.expectOK(t, "GET", "/api/auth/users", nil)
	if len(dataList(t, m)) < 1 {
		t.Fatalf("用户列表为空")
	}

	// 22. 只读访客权限
	m = c.expectOK(t, "POST", "/api/auth/users", map[string]interface{}{
		"username": "g1", "real_name": "访客", "role": "guest", "password": "123456",
	})
	guestID := num(dataMap(t, m)["id"])
	g := &apiClient{base: base}
	g.login(t, "g1", "123456")
	g.expectOK(t, "GET", "/api/base/subjects", nil) // 可查看
	g.expectStatus(t, 403, "POST", "/api/base/grades", map[string]interface{}{"name": "二年级"})
	g.expectStatus(t, 403, "POST", "/api/schedule/run", nil)
	g.expectStatus(t, 403, "GET", "/api/backup", nil)
	g.expectStatus(t, 403, "GET", "/api/auth/users", nil)
	g.expectStatus(t, 403, "GET", "/api/export/all-classes/excel", nil)
	// 删除访客后会话失效
	c.expectOK(t, "DELETE", fmt.Sprintf("/api/auth/users/%d", guestID), nil)
	g.expectStatus(t, 401, "GET", "/api/auth/me", nil)

	// 23. 操作日志
	m = c.expectOK(t, "GET", "/api/logs?per_page=100", nil)
	found := false
	for _, l := range dataList(t, m) {
		if mm := l.(map[string]interface{}); mm["action"] == "login" {
			found = true
		}
	}
	if !found {
		t.Fatalf("日志中未找到 login 记录")
	}
	m = c.expectOK(t, "GET", "/api/logs/actions", nil)
	if dataOf(t, m) == nil {
		t.Fatalf("日志操作名缺失")
	}

	// 24. 修改密码（改回原密码，保持 admin/admin123 可用）
	c.expectOK(t, "POST", "/api/auth/change-password", map[string]interface{}{"old_password": "admin123", "new_password": "admin1234"})
	c.expectOK(t, "POST", "/api/auth/change-password", map[string]interface{}{"old_password": "admin1234", "new_password": "admin123"})

	t.Logf("✅ 全部端到端测试通过：排课 %d 节、零冲突、导出/备份/权限均正常", totalHours)
}
