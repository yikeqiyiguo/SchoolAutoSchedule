package store

import (
	"database/sql"
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
)

// SeedUsers 默认账号
type SeedUser struct {
	Username, Password, RealName, Role string
}

var defaultUsers = []SeedUser{
	{Username: "admin", Password: "admin123", RealName: "系统管理员", Role: "super"},
}

// DefaultSubjects 默认科目（subject_type: main/sub）
var DefaultSubjects = []struct{ Name, Type string }{
	{"语文", "main"}, {"数学", "main"}, {"英语", "main"}, {"物理", "main"},
	{"化学", "main"}, {"生物", "main"}, {"历史", "sub"}, {"地理", "sub"},
	{"政治", "sub"}, {"体育", "sub"}, {"音乐", "sub"}, {"美术", "sub"},
	{"信息科技", "sub"}, {"道德与法治", "sub"}, {"劳动", "sub"}, {"班会", "sub"},
	{"心理", "sub"}, {"科学", "sub"},
}

// DefaultPeriods 默认课时时段
var DefaultPeriods = [][3]string{
	{"08:00", "08:45", "morning"}, {"08:55", "09:40", "morning"},
	{"10:00", "10:45", "morning"}, {"10:55", "11:40", "morning"},
	{"14:00", "14:45", "afternoon"}, {"14:55", "15:40", "afternoon"},
	{"16:00", "16:45", "afternoon"}, {"16:55", "17:40", "afternoon"},
	{"19:00", "19:45", "evening"}, {"19:55", "20:40", "evening"},
}

// DefaultRules 默认软约束规则
type DefaultRule struct {
	Type, Name, Desc  string
	Priority          int
	Params            map[string]interface{}
}

var DefaultRules = []DefaultRule{
	{"morning_subject", "主科优先上午", "语文/数学/英语等主科课程优先安排到上午时段（最佳学习时段）", 95, map[string]interface{}{"subject_ids": []int{1, 2, 3}}},
	{"evening_limit", "副科不排晚自习", "体育/音乐/美术等非主科不安排到晚自习时段", 90, map[string]interface{}{}},
	{"avoid_back_to_back", "避免同科目连排", "同一班级同一天同一科目不超过设置节数", 75, map[string]interface{}{"max_consecutive": 2}},
	{"avoid_overload", "教师每日不超负荷", "同一位教师同一天课程不超过设置节数", 85, map[string]interface{}{"max_consecutive": 4}},
	{"teacher_morning_balance", "教师上午课均衡", "各教师上午课程节数尽量均衡", 70, map[string]interface{}{}},
	{"subject_stable", "科目日排课均衡", "同一科目在一周内分布尽量均匀", 65, map[string]interface{}{"subject_ids": []int{1, 2, 3}}},
	{"class_teacher_first", "班主任前两节", "班主任上午时段课程优先安排", 60, map[string]interface{}{"period_types": []string{"morning"}}},
}

// RuleTypeMeta 规则类型元信息（供前端渲染参数表单）
type RuleTypeMeta struct {
	Name   string                    `json:"name"`
	Desc   string                    `json:"desc"`
	Params []RuleTypeParamMeta       `json:"params"`
}

type RuleTypeParamMeta struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// RuleTypes 规则类型注册表
var RuleTypes = map[string]RuleTypeMeta{
	"morning_subject":      {"主科优先上午", "主科课程优先安排到上午时段（最佳学习时段）", []RuleTypeParamMeta{{"subject_ids", "适用科目（留空则按主科类型自动生效）"}}},
	"evening_limit":        {"副科不排晚自习", "体育/音乐/美术等非主科不安排到晚自习时段", nil},
	"avoid_back_to_back":   {"避免同科目连排", "同一班级同一天同一科目不超过设置节数", []RuleTypeParamMeta{{"max_consecutive", "同天最多连续节数"}}},
	"avoid_overload":       {"教师每日不超负荷", "同一位教师同一天课程不超过设置节数", []RuleTypeParamMeta{{"max_consecutive", "每日最多节数"}}},
	"teacher_morning_balance": {"教师上午课均衡", "各教师上午课程节数尽量均衡", nil},
	"subject_stable":       {"科目日排课均衡", "同一科目在一周内分布尽量均匀", []RuleTypeParamMeta{{"subject_ids", "适用科目（留空则全部科目）"}}},
	"class_teacher_first":  {"班主任前两节", "班主任上午时段课程优先安排", []RuleTypeParamMeta{{"period_types", "优先时段"}}},
}

// PeriodTypeNames 时段类型名称
var PeriodTypeNames = map[string]string{
	"morning": "上午", "afternoon": "下午", "evening": "晚间（晚自习）",
}

// ActionNames 操作日志类型名称
var ActionNames = map[string]string{
	"login": "登录", "logout": "退出", "create": "新增", "update": "修改",
	"delete": "删除", "import": "导入", "export": "导出", "schedule": "排课",
	"backup": "备份", "restore": "恢复", "reset": "重置",
}

func seedIfEmpty() error {
	// 系统配置
	var n int
	if err := DB.QueryRow("SELECT COUNT(*) FROM system_config").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := DB.Exec("INSERT INTO system_config (id, school_name, school_days) VALUES (1, '', '1,2,3,4,5')"); err != nil {
			return err
		}
	}
	// 默认用户
	if err := DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		for _, u := range defaultUsers {
			hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			if _, err := DB.Exec("INSERT INTO users (username, password_hash, real_name, role) VALUES (?,?,?,?)",
				u.Username, string(hash), u.RealName, u.Role); err != nil {
				return err
			}
		}
	}
	// 默认科目
	if err := DB.QueryRow("SELECT COUNT(*) FROM subjects").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		for i, s := range DefaultSubjects {
			if _, err := DB.Exec("INSERT INTO subjects (name, subject_type, sort_order, is_default) VALUES (?,?,?,1)",
				s.Name, s.Type, i+1); err != nil {
				return err
			}
		}
	}
	// 默认时段
	if err := DB.QueryRow("SELECT COUNT(*) FROM periods").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		for i, p := range DefaultPeriods {
			if _, err := DB.Exec("INSERT INTO periods (period_index, start_time, end_time, period_type) VALUES (?,?,?,?)",
				i+1, p[0], p[1], p[2]); err != nil {
				return err
			}
		}
	}
	// 默认规则
	if err := DB.QueryRow("SELECT COUNT(*) FROM rules").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		for _, r := range DefaultRules {
			params, err := json.Marshal(r.Params)
			if err != nil {
				return err
			}
			if _, err := DB.Exec("INSERT INTO rules (name, rule_type, priority, is_default, params, description) VALUES (?,?,?,1,?,?)",
				r.Name, r.Type, r.Priority, string(params), r.Desc); err != nil {
				return err
			}
		}
	}
	return nil
}

// DB 便捷查询（供其他包使用，避免重复 import database/sql）
func GetDB() *sql.DB { return DB }
