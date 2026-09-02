package api

// routes 注册全部路由（Go 1.22+ 方法+通配符路由）
func (s *Server) routes() {
	// 认证
	s.mux.HandleFunc("POST /api/auth/login", handleLogin)
	s.mux.HandleFunc("POST /api/auth/logout", handleLogout)
	s.mux.HandleFunc("GET /api/auth/me", handleMe)
	s.mux.HandleFunc("POST /api/auth/change-password", handleChangePassword)
	s.mux.HandleFunc("GET /api/auth/users", handleUsers)
	s.mux.HandleFunc("POST /api/auth/users", handleUsers)
	s.mux.HandleFunc("PUT /api/auth/users/{id}", handleUserUpdate)
	s.mux.HandleFunc("DELETE /api/auth/users/{id}", handleUserUpdate)
	s.mux.HandleFunc("POST /api/auth/reset-password/{id}", handleResetPassword)

	// 系统配置
	s.mux.HandleFunc("GET /api/config/system", handleGetSystem)
	s.mux.HandleFunc("PUT /api/config/system", handlePutSystem)
	s.mux.HandleFunc("GET /api/config/periods", handleGetPeriods)
	s.mux.HandleFunc("PUT /api/config/periods", handlePutPeriods)
	s.mux.HandleFunc("GET /api/config/meta", handleMeta)

	// 基础数据
	s.mux.HandleFunc("GET /api/base/grades", handleGrades)
	s.mux.HandleFunc("POST /api/base/grades", handleGrades)
	s.mux.HandleFunc("PUT /api/base/grades/{id}", handleGradeUpdate)
	s.mux.HandleFunc("DELETE /api/base/grades/{id}", handleGradeUpdate)
	s.mux.HandleFunc("GET /api/base/classes", handleClasses)
	s.mux.HandleFunc("POST /api/base/classes", handleClasses)
	s.mux.HandleFunc("PUT /api/base/classes/{id}", handleClassUpdate)
	s.mux.HandleFunc("DELETE /api/base/classes/{id}", handleClassUpdate)
	s.mux.HandleFunc("GET /api/base/subjects", handleSubjects)
	s.mux.HandleFunc("POST /api/base/subjects", handleSubjects)
	s.mux.HandleFunc("PUT /api/base/subjects/{id}", handleSubjectUpdate)
	s.mux.HandleFunc("DELETE /api/base/subjects/{id}", handleSubjectUpdate)
	s.mux.HandleFunc("GET /api/base/teachers", handleTeachers)
	s.mux.HandleFunc("POST /api/base/teachers", handleTeachers)
	s.mux.HandleFunc("PUT /api/base/teachers/{id}", handleTeacherUpdate)
	s.mux.HandleFunc("DELETE /api/base/teachers/{id}", handleTeacherUpdate)
	s.mux.HandleFunc("GET /api/base/assignments", handleAssignments)
	s.mux.HandleFunc("POST /api/base/assignments", handleAssignments)
	s.mux.HandleFunc("PUT /api/base/assignments/{id}", handleAssignmentUpdate)
	s.mux.HandleFunc("DELETE /api/base/assignments/{id}", handleAssignmentUpdate)
	s.mux.HandleFunc("POST /api/base/assignments/clear", handleAssignmentsClear)
	s.mux.HandleFunc("GET /api/base/import/template/{kind}", handleImportTemplate)
	s.mux.HandleFunc("POST /api/base/import/{kind}", handleImport)

	// 排课规则
	s.mux.HandleFunc("GET /api/rules", handleRules)
	s.mux.HandleFunc("POST /api/rules", handleRules)
	s.mux.HandleFunc("PUT /api/rules/{id}", handleRuleUpdate)
	s.mux.HandleFunc("DELETE /api/rules/{id}", handleRuleUpdate)

	// 排课
	s.mux.HandleFunc("GET /api/schedule/overview", handleScheduleOverview)
	s.mux.HandleFunc("POST /api/schedule/run", handleScheduleRun)
	s.mux.HandleFunc("POST /api/schedule/clear", handleScheduleClear)
	s.mux.HandleFunc("GET /api/schedule/conflicts", handleScheduleConflicts)
	s.mux.HandleFunc("POST /api/schedule/cell", handleScheduleCell)
	s.mux.HandleFunc("POST /api/schedule/swap", handleScheduleSwap)
	s.mux.HandleFunc("GET /api/schedule/timetable/class", handleClassTimetable)
	s.mux.HandleFunc("GET /api/schedule/timetable/teacher", handleTeacherTimetable)

	// 统计
	s.mux.HandleFunc("GET /api/stats/overview", handleStatsOverview)
	s.mux.HandleFunc("GET /api/stats/teacher", handleStatsTeacher)
	s.mux.HandleFunc("GET /api/stats/class", handleStatsClass)

	// 日志
	s.mux.HandleFunc("GET /api/logs", handleLogs)
	s.mux.HandleFunc("GET /api/logs/actions", handleLogActions)

	// 导出
	s.mux.HandleFunc("GET /api/export/all-classes/excel", handleExportAllClasses)
	s.mux.HandleFunc("GET /api/export/all-teachers/excel", handleExportAllTeachers)
	s.mux.HandleFunc("GET /api/export/class/{id}/excel", handleExportClassExcel)
	s.mux.HandleFunc("GET /api/export/class/{id}/pdf", handleExportClassPDF)
	s.mux.HandleFunc("GET /api/export/teacher/{id}/excel", handleExportTeacherExcel)
	s.mux.HandleFunc("GET /api/export/teacher/{id}/pdf", handleExportTeacherPDF)
	s.mux.HandleFunc("GET /api/export/stats/excel", handleExportStats)

	// 学校管理（多学校独立课表）
	s.mux.HandleFunc("GET /api/schools", handleSchools)
	s.mux.HandleFunc("POST /api/schools", handleSchools)
	s.mux.HandleFunc("PUT /api/schools/{id}", handleSchoolUpdate)
	s.mux.HandleFunc("DELETE /api/schools/{id}", handleSchoolUpdate)
	s.mux.HandleFunc("POST /api/schools/{id}/switch", handleSchoolSwitch)

	// 备份
	s.mux.HandleFunc("GET /api/backup", handleBackupList)
	s.mux.HandleFunc("POST /api/backup/create", handleBackupCreate)
	s.mux.HandleFunc("POST /api/backup/restore/{id}", handleBackupRestore)
	s.mux.HandleFunc("DELETE /api/backup/delete/{id}", handleBackupDelete)
	s.mux.HandleFunc("GET /api/backup/download/{id}", handleBackupDownload)

	// 静态资源
	s.mux.HandleFunc("/", handleStatic)
}
