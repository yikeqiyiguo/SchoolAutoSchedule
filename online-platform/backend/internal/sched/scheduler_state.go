package sched

import (
	"strconv"

	"school-scheduler/internal/store"
)

// newState 初始化运行状态
func newState(periods []periodRow) *state {
	pt := map[int]string{}
	lastRegular := 0
	for _, p := range periods {
		pt[p.Index] = p.PeriodType
		// "末节"取每天最后一个常规节次（非晚自习）；晚自习由 evening_limit 规则约束
		if p.PeriodType != "evening" {
			lastRegular = p.Index
		}
	}
	return &state{
		classGrid:         map[int]map[int]map[int]*Place{},
		teacherGrid:       map[int]map[int]map[int][]*Place{},
		teacherWeekCount:  map[int]int{},
		teacherDayCount:   map[int]map[int]int{},
		teacherMorningCnt: map[int]int{},
		classSubjectDay:   map[int]map[int]map[int]int{},
		subjectSubjectDay: map[int]map[int]map[int]int{},
		periodType:        pt,
		lastPeriod:        lastRegular,
	}
}

// loadManual 载入手动固定课作为占位
func (st *state) loadManual() {
	rows, err := store.DB.Query(
		`SELECT tt.class_id, tt.day_index, tt.period_index, tt.subject_id, tt.teacher_id, s.subject_type, t.name, COALESCE(p.period_type,'morning')
		 FROM timetable tt
		 JOIN subjects s ON tt.subject_id=s.id
		 JOIN teachers t ON tt.teacher_id=t.id
		 LEFT JOIN periods p ON tt.period_index=p.period_index
		 WHERE tt.source='manual'`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var cid, d, p, sid, tid int
		var stype, tname, pt string
		if err := rows.Scan(&cid, &d, &p, &sid, &tid, &stype, &tname, &pt); err != nil {
			continue
		}
		l := &Lesson{
			ClassID: cid, SubjectID: sid, SubjectType: stype,
			TeacherID: tid, TeacherName: tname,
			AllowEvening: true, TeacherLimit: 9999, Manual: true,
		}
		st.place(l, Slot{Day: d, Period: p, PeriodType: pt}, false)
	}
}

// save 将排课结果写入数据库
func (st *state) save(keepManual bool) error {
	tx, err := store.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if keepManual {
		if _, err := tx.Exec("DELETE FROM timetable WHERE source='auto'"); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec("DELETE FROM timetable"); err != nil {
			return err
		}
	}
	for _, days := range st.teacherGrid {
		for _, day := range days {
			for _, list := range day {
				for _, pl := range list {
					if pl.Lesson.Manual {
						continue
					}
					if _, err := tx.Exec(
						"INSERT INTO timetable (class_id, day_index, period_index, subject_id, teacher_id, source) VALUES (?,?,?,?,?,?)",
						pl.Lesson.ClassID, pl.Day, pl.Period, pl.Lesson.SubjectID, pl.Lesson.TeacherID, "auto"); err != nil {
						return err
					}
				}
			}
		}
	}
	return tx.Commit()
}

// dayName 星期名称
func dayName(d int) string {
	return map[int]string{1: "周一", 2: "周二", 3: "周三", 4: "周四", 5: "周五", 6: "周六", 7: "周日"}[d]
}

// itoa int 转字符串
func itoa(i int) string { return strconv.Itoa(i) }
