// Package sched 实现一键排课算法：贪心放置 + 软约束打分 + 冲突兜底
package sched

import (
	"encoding/json"
	"math/rand"
	"sort"
	"time"

	"school-scheduler/internal/store"
)

// Lesson 一节待排课程
type Lesson struct {
	Index          int
	ClassID        int
	SubjectID      int
	SubjectType    string // main/sub
	SubjectName    string
	TeacherID      int
	TeacherName    string
	PreferMorning  bool
	AllowEvening   bool
	IsClassTeacher bool
	TeacherLimit   int
	Manual         bool // 手动固定课（不参与自动重排）
}

// Slot 一个可排位置
type Slot struct {
	Day        int
	Period     int
	PeriodType string
}

// Place 已放置记录（PeriodType 用于撤销计数）
type Place struct {
	Lesson     *Lesson
	Day        int
	Period     int
	PeriodType string
	Conflict   bool // 冲突放置（硬约束被放宽）
}

// SoftRule 软约束规则
type SoftRule struct {
	Type           string
	Priority       int
	MaxConsecutive int
	SubjectIDs     map[int]bool // 空 = 全部
	TeacherIDs     map[int]bool
}

// Conflict 冲突项
type Conflict struct {
	Type    string `json:"type"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// Result 排课结果
type Result struct {
	Success   bool       `json:"success"`
	Message   string     `json:"message"`
	CostMS    int64      `json:"cost_ms"`
	Placed    int        `json:"placed"`
	Conflicts []Conflict `json:"conflicts"`
	Errors    []string   `json:"errors"`
}

// 运行状态
type state struct {
	classGrid         map[int]map[int]map[int]*Place // 班级唯一（单指针）
	teacherGrid       map[int]map[int]map[int][]*Place // 教师可多占（宽松时）
	teacherWeekCount  map[int]int
	teacherDayCount   map[int]map[int]int
	teacherMorningCnt map[int]int
	classSubjectDay   map[int]map[int]map[int]int // class->subject->day 计数
	subjectSubjectDay map[int]map[int]map[int]int // class->subject->day 集合
}

// Run 执行排课。keepManual 为 true 时保留手动课。
func Run(keepManual bool) (*Result, error) {
	start := time.Now()
	res := &Result{Message: "排课完成", Success: true}

	// 1. 读取基础数据
	periods, err := loadPeriods()
	if err != nil {
		return nil, err
	}
	if len(periods) == 0 {
		return nil, errNoPeriods
	}
	schoolDays, err := loadSchoolDays()
	if err != nil || len(schoolDays) == 0 {
		schoolDays = []int{1, 2, 3, 4, 5}
	}
	teachers := loadTeachers()
	classes := loadClasses()
	lessons, err := buildLessons(classes, teachers)
	if err != nil {
		return nil, err
	}
	softRules, err := loadSoftRules()
	if err != nil {
		return nil, err
	}

	// 2. 生成全部可用 slot
	slots := make([]Slot, 0, len(schoolDays)*len(periods))
	for _, d := range schoolDays {
		for _, p := range periods {
			slots = append(slots, Slot{Day: d, Period: p.Index, PeriodType: p.PeriodType})
		}
	}
	if len(slots) == 0 {
		return nil, errNoPeriods
	}

	// 3. 初始化状态（含手动课占位）
	st := newState(periods)
	if keepManual {
		st.loadManual()
	}

	// 4. 约束紧的优先
	sort.SliceStable(lessons, func(i, j int) bool {
		return lessonPriority(lessons[i]) > lessonPriority(lessons[j])
	})

	// 5. 贪心放置（多轮，每轮带随机扰动）
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for pass := 0; pass < 3; pass++ {
		moved := false
		for _, l := range lessons {
			if st.findPlace(l) != nil {
				continue
			}
			best := st.bestSlot(l, slots, softRules, rng, true)
			if best.Day == 0 {
				best = st.bestSlot(l, slots, softRules, rng, false)
				if best.Day == 0 {
					res.Errors = append(res.Errors, "课时无法安排位置")
					continue
				}
				st.place(l, best, true)
			} else {
				st.place(l, best, false)
			}
			moved = true
		}
		if !moved {
			break
		}
	}

	// 6. 冲突修复（交换调整）
	st.repairLoop(lessons, slots, softRules, rng)

	res.CostMS = time.Since(start).Milliseconds()
	res.Placed = st.placedCount()

	// 7. 统计冲突
	res.Conflicts = st.detectConflicts()
	if len(res.Conflicts) > 0 {
		res.Success = false
		res.Message = "排课完成，但存在" + itoa(len(res.Conflicts)) + "项冲突，请在「冲突检测」中查看并微调"
	}

	// 8. 写入数据库
	if err := st.save(keepManual); err != nil {
		return nil, err
	}
	return res, nil
}

func lessonPriority(l *Lesson) int {
	p := 0
	if l.SubjectType == "main" {
		p += 4
	}
	if l.PreferMorning {
		p += 2
	}
	if !l.AllowEvening {
		p += 3
	}
	if l.TeacherLimit < 20 {
		p += 1
	}
	return p
}

func (st *state) bestSlot(l *Lesson, slots []Slot, rules []*SoftRule, rng *rand.Rand, strict bool) Slot {
	var best Slot
	bestScore := -1 << 30
	for _, s := range slots {
		if strict {
			if !st.canPlaceStrict(l, s) {
				continue
			}
		} else if !st.canPlaceClassDay(l, s) {
			continue
		}
		sc := st.scoreSlot(l, s, rules) + rng.Intn(6)
		if sc > bestScore {
			bestScore = sc
			best = s
		}
	}
	return best
}

/* ---------- 硬约束 ---------- */

func (st *state) canPlaceStrict(l *Lesson, s Slot) bool {
	if st.classGrid[l.ClassID][s.Day][s.Period] != nil {
		return false
	}
	if len(st.teacherGrid[l.TeacherID][s.Day][s.Period]) > 0 {
		return false
	}
	if st.teacherWeekCount[l.TeacherID]+1 > l.TeacherLimit {
		return false
	}
	if !l.AllowEvening && s.PeriodType == "evening" {
		return false
	}
	return true
}

// canPlaceClassDay 宽松模式仍保证班级唯一
func (st *state) canPlaceClassDay(l *Lesson, s Slot) bool {
	return st.classGrid[l.ClassID][s.Day][s.Period] == nil
}

/* ---------- 软约束打分 ---------- */

func (st *state) scoreSlot(l *Lesson, s Slot, rules []*SoftRule) int {
	score := 0
	for _, r := range rules {
		switch r.Type {
		case "morning_subject":
			if ruleApplies(r, l) && l.SubjectType == "main" && s.PeriodType == "morning" {
				score += 8
			}
		case "evening_limit":
			if l.SubjectType == "sub" && s.PeriodType == "evening" {
				score -= 40
			}
		case "avoid_back_to_back":
			maxN := r.MaxConsecutive
			if maxN <= 0 {
				maxN = 2
			}
			if ruleApplies(r, l) {
				cnt := st.classSubjectDay[l.ClassID][l.SubjectID][s.Day]
				if cnt >= maxN {
					score -= 15
				} else {
					score += 3
				}
			}
		case "avoid_overload":
			maxN := r.MaxConsecutive
			if maxN <= 0 {
				maxN = 4
			}
			if ruleApplies(r, l) {
				cnt := st.teacherDayCount[l.TeacherID][s.Day]
				if cnt >= maxN {
					score -= 20
				} else {
					score += 2
				}
			}
		case "teacher_morning_balance":
			if ruleApplies(r, l) {
				score -= st.teacherMorningCnt[l.TeacherID]
			}
		case "subject_stable":
			if ruleApplies(r, l) {
				score += 5 - len(st.subjectSubjectDay[l.ClassID][l.SubjectID])
			}
		case "class_teacher_first":
			if l.IsClassTeacher && s.PeriodType == "morning" {
				score += 5
			}
		}
	}
	if l.PreferMorning && s.PeriodType == "morning" {
		score += 4
	}
	if st.teacherDayCount[l.TeacherID][s.Day] < 3 {
		score += 1
	}
	return score
}

func ruleApplies(r *SoftRule, l *Lesson) bool {
	if len(r.SubjectIDs) > 0 && !r.SubjectIDs[l.SubjectID] {
		return false
	}
	if len(r.TeacherIDs) > 0 && !r.TeacherIDs[l.TeacherID] {
		return false
	}
	return true
}

/* ---------- 放置与状态维护 ---------- */

func (st *state) place(l *Lesson, s Slot, conflict bool) {
	p := &Place{Lesson: l, Day: s.Day, Period: s.Period, PeriodType: s.PeriodType, Conflict: conflict}
	// 班级（唯一）
	if st.classGrid[l.ClassID] == nil {
		st.classGrid[l.ClassID] = map[int]map[int]*Place{}
	}
	if st.classGrid[l.ClassID][s.Day] == nil {
		st.classGrid[l.ClassID][s.Day] = map[int]*Place{}
	}
	st.classGrid[l.ClassID][s.Day][s.Period] = p
	// 教师（可多占）
	if st.teacherGrid[l.TeacherID] == nil {
		st.teacherGrid[l.TeacherID] = map[int]map[int][]*Place{}
	}
	if st.teacherGrid[l.TeacherID][s.Day] == nil {
		st.teacherGrid[l.TeacherID][s.Day] = map[int][]*Place{}
	}
	st.teacherGrid[l.TeacherID][s.Day][s.Period] = append(st.teacherGrid[l.TeacherID][s.Day][s.Period], p)
	// 计数
	st.teacherWeekCount[l.TeacherID]++
	if st.teacherDayCount[l.TeacherID] == nil {
		st.teacherDayCount[l.TeacherID] = map[int]int{}
	}
	st.teacherDayCount[l.TeacherID][s.Day]++
	if s.PeriodType == "morning" {
		st.teacherMorningCnt[l.TeacherID]++
	}
	if st.classSubjectDay[l.ClassID] == nil {
		st.classSubjectDay[l.ClassID] = map[int]map[int]int{}
	}
	if st.classSubjectDay[l.ClassID][l.SubjectID] == nil {
		st.classSubjectDay[l.ClassID][l.SubjectID] = map[int]int{}
	}
	st.classSubjectDay[l.ClassID][l.SubjectID][s.Day]++
	if st.subjectSubjectDay[l.ClassID] == nil {
		st.subjectSubjectDay[l.ClassID] = map[int]map[int]int{}
	}
	if st.subjectSubjectDay[l.ClassID][l.SubjectID] == nil {
		st.subjectSubjectDay[l.ClassID][l.SubjectID] = map[int]int{}
	}
	st.subjectSubjectDay[l.ClassID][l.SubjectID][s.Day]++
}

func (st *state) remove(l *Lesson) {
	// 教师网格
	for d, day := range st.teacherGrid[l.TeacherID] {
		for p, list := range day {
			for i, pl := range list {
				if pl.Lesson == l {
					list = append(list[:i], list[i+1:]...)
					day[p] = list
					st.teacherWeekCount[l.TeacherID]--
					st.teacherDayCount[l.TeacherID][d]--
					if pl.PeriodType == "morning" {
						st.teacherMorningCnt[l.TeacherID]--
					}
					break
				}
			}
		}
	}
	// 班级网格
	for d, day := range st.classGrid[l.ClassID] {
		for p, pl := range day {
			if pl.Lesson == l {
				delete(day, p)
				if st.classSubjectDay[l.ClassID][l.SubjectID] != nil {
					st.classSubjectDay[l.ClassID][l.SubjectID][d]--
				}
				if st.subjectSubjectDay[l.ClassID][l.SubjectID] != nil {
					delete(st.subjectSubjectDay[l.ClassID][l.SubjectID], d)
				}
				break
			}
		}
	}
}

func (st *state) findPlace(l *Lesson) *Place {
	for _, day := range st.teacherGrid[l.TeacherID] {
		for _, list := range day {
			for _, pl := range list {
				if pl.Lesson == l {
					return pl
				}
			}
		}
	}
	return nil
}

/* ---------- 冲突修复 ---------- */

func (st *state) repairLoop(lessons []*Lesson, slots []Slot, rules []*SoftRule, rng *rand.Rand) {
	conflictLessons := st.conflictLessons()
	for iter := 0; iter < 3000 && len(conflictLessons) > 0; iter++ {
		l := conflictLessons[rng.Intn(len(conflictLessons))]
		st.remove(l)
		best := st.bestSlot(l, slots, rules, rng, true)
		if best.Day == 0 {
			best = st.bestSlot(l, slots, rules, rng, false)
			if best.Day != 0 {
				st.place(l, best, true)
			}
		} else {
			st.place(l, best, false)
		}
		conflictLessons = st.conflictLessons()
	}
}

// conflictLessons 返回所有冲突放置的课程
func (st *state) conflictLessons() []*Lesson {
	set := map[*Lesson]bool{}
	// 教师撞课
	for _, days := range st.teacherGrid {
		for _, day := range days {
			for _, list := range day {
				if len(list) > 1 {
					for _, pl := range list {
						set[pl.Lesson] = true
					}
				}
			}
		}
	}
	out := []*Lesson{}
	for l := range set {
		out = append(out, l)
	}
	return out
}

/* ---------- 冲突检测 ---------- */

func (st *state) detectConflicts() []Conflict {
	list := []Conflict{}
	seen := map[string]bool{}
	// 教师撞课
	for tid, days := range st.teacherGrid {
		for d, day := range days {
			for p, plist := range day {
				if len(plist) > 1 {
					manualOnly := true
					for _, pl := range plist {
						if !pl.Lesson.Manual {
							manualOnly = false
						}
					}
					if manualOnly {
						continue
					}
					msg := "教师「" + teacherNames[tid] + "」" + dayName(d) + "第" + itoa(p) + "节同时安排了" + itoa(len(plist)) + "节课（撞课）"
					list = append(list, Conflict{
						Type: "teacher_conflict", Level: "error", Message: msg,
					})
				}
			}
		}
	}
	// 班级撞课
	for cid, days := range st.classGrid {
		for d, day := range days {
			for p, pl := range day {
				if pl.Conflict {
					key := itoa(cid) + "-" + itoa(d) + "-" + itoa(p)
					if !seen[key] {
						seen[key] = true
						list = append(list, Conflict{
							Type: "class_conflict", Level: "warning",
							Message: "班级课时安排在冲突位置（天" + itoa(d) + "节" + itoa(p) + "）",
						})
					}
				}
			}
		}
	}
	// 教师超课时
	for tid, cnt := range st.teacherWeekCount {
		if limit := teacherLimits[tid]; limit > 0 && cnt > limit {
			list = append(list, Conflict{
				Type: "hour_limit", Level: "warning",
				Message: "教师「" + teacherNames[tid] + "」本周" + itoa(cnt) + "节，超过上限" + itoa(limit) + "节",
			})
		}
	}
	// 班级课时不足（应排未排）
	need := map[int]int{}
	have := map[int]int{}
	for _, l := range st.allLessons() {
		need[l.ClassID] += 1
	}
	for cid, days := range st.classGrid {
		for _, day := range days {
			have[cid] += len(day)
		}
	}
	for cid, n := range need {
		if h := have[cid]; h < n {
			list = append(list, Conflict{
				Type: "hour_mismatch", Level: "warning",
				Message: "班级课时不足：应排" + itoa(n) + "节，实际" + itoa(h) + "节",
			})
		}
	}
	return list
}

func (st *state) allLessons() []*Lesson {
	set := map[*Lesson]bool{}
	for _, days := range st.teacherGrid {
		for _, day := range days {
			for _, list := range day {
				for _, pl := range list {
					set[pl.Lesson] = true
				}
			}
		}
	}
	out := []*Lesson{}
	for l := range set {
		out = append(out, l)
	}
	return out
}

func (st *state) placedCount() int {
	return len(st.allLessons())
}

/* ---------- 数据读取 ---------- */

type periodRow struct {
	Index      int
	PeriodType string
}

var errNoPeriods = errString("未配置课时时段")

type errString string

func (e errString) Error() string { return string(e) }

func loadPeriods() ([]periodRow, error) {
	rows, err := store.DB.Query("SELECT period_index, period_type FROM periods ORDER BY period_index")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []periodRow{}
	for rows.Next() {
		var pr periodRow
		if err := rows.Scan(&pr.Index, &pr.PeriodType); err == nil {
			out = append(out, pr)
		}
	}
	return out, nil
}

func loadSchoolDays() ([]int, error) {
	var s string
	if err := store.DB.QueryRow("SELECT school_days FROM system_config WHERE id=1").Scan(&s); err != nil {
		return []int{1, 2, 3, 4, 5}, nil
	}
	out := []int{}
	for _, c := range s {
		if c >= '1' && c <= '7' {
			out = append(out, int(c-'0'))
		}
	}
	return out, nil
}

var (
	teacherNames  map[int]string
	teacherLimits map[int]int
)

func loadTeachers() map[int]map[string]interface{} {
	m := map[int]map[string]interface{}{}
	teacherNames = map[int]string{}
	teacherLimits = map[int]int{}
	rows, _ := store.DB.Query("SELECT id, name, weekly_hour_limit, allow_evening, is_class_teacher FROM teachers WHERE enabled=1")
	defer rows.Close()
	for rows.Next() {
		var id, limit int
		var name string
		var allow, isCT bool
		rows.Scan(&id, &name, &limit, &allow, &isCT)
		teacherNames[id] = name
		teacherLimits[id] = limit
		m[id] = map[string]interface{}{
			"name": name, "limit": limit, "allow_evening": allow, "is_class_teacher": isCT,
		}
	}
	return m
}

func loadClasses() []int {
	out := []int{}
	rows, _ := store.DB.Query("SELECT id FROM classes WHERE enabled=1 ORDER BY grade_id, class_no")
	defer rows.Close()
	for rows.Next() {
		var id int
		rows.Scan(&id)
		out = append(out, id)
	}
	return out
}

func buildLessons(classes []int, teachers map[int]map[string]interface{}) ([]*Lesson, error) {
	if len(classes) == 0 {
		return nil, errString("没有可排课的班级，请先在基础数据中新增班级")
	}
	out := []*Lesson{}
	idx := 0
	for _, cid := range classes {
		rows, err := store.DB.Query(
			"SELECT a.subject_id, a.teacher_id, a.weekly_hours, a.prefer_morning, s.subject_type, s.name "+
				"FROM assignments a JOIN subjects s ON a.subject_id=s.id WHERE a.class_id=? AND s.enabled=1",
			cid)
		if err != nil {
			continue
		}
		for rows.Next() {
			var sid, tid, wh int
			var st, sname string
			var pm bool
			if err := rows.Scan(&sid, &tid, &wh, &pm, &st, &sname); err != nil {
				continue
			}
			tInfo, ok := teachers[tid]
			if !ok {
				continue
			}
			limit := tInfo["limit"].(int)
			allowEve := tInfo["allow_evening"].(bool)
			isCT := tInfo["is_class_teacher"].(bool)
			for i := 0; i < wh; i++ {
				idx++
				out = append(out, &Lesson{
					Index: idx, ClassID: cid, SubjectID: sid, SubjectType: st, SubjectName: sname,
					TeacherID: tid, TeacherName: tInfo["name"].(string),
					PreferMorning: pm, AllowEvening: allowEve, IsClassTeacher: isCT, TeacherLimit: limit,
				})
			}
		}
		rows.Close()
	}
	return out, nil
}

func loadSoftRules() ([]*SoftRule, error) {
	rows, err := store.DB.Query("SELECT rule_type, priority, params FROM rules WHERE enabled=1 ORDER BY priority DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*SoftRule{}
	for rows.Next() {
		var rtype string
		var prio int
		var params string
		rows.Scan(&rtype, &prio, &params)
		r := &SoftRule{Type: rtype, Priority: prio}
		m := map[string]interface{}{}
		json.Unmarshal([]byte(params), &m)
		if v, ok := m["max_consecutive"].(float64); ok {
			r.MaxConsecutive = int(v)
		}
		if v, ok := m["subject_ids"].([]interface{}); ok {
			r.SubjectIDs = map[int]bool{}
			for _, x := range v {
				r.SubjectIDs[int(x.(float64))] = true
			}
		}
		if v, ok := m["teacher_ids"].([]interface{}); ok {
			r.TeacherIDs = map[int]bool{}
			for _, x := range v {
				r.TeacherIDs[int(x.(float64))] = true
			}
		}
		out = append(out, r)
	}
	return out, nil
}
