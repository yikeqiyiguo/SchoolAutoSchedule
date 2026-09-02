package api

import (
	"net/http"

	"school-scheduler/internal/store"
)

/* ---------- 学校管理（多学校独立课表） ---------- */

type schoolJSON struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	Current   bool   `json:"current"`
}

// handleSchools GET 列表 / POST 新建（新建后自动切换）
func handleSchools(w http.ResponseWriter, r *http.Request) {
	u := requireLogin(w, r)
	if u == nil {
		return
	}
	switch r.Method {
	case "GET":
		list := []schoolJSON{}
		for _, s := range store.ListSchools() {
			list = append(list, schoolJSON{
				ID: s.ID, Name: s.Name, CreatedAt: s.CreatedAt,
				Current: s.ID == store.CurrentSchoolID,
			})
		}
		ok(w, map[string]interface{}{
			"list": list,
			"current": map[string]interface{}{
				"id": store.CurrentSchoolID, "name": store.CurrentSchoolName,
			},
		})
	case "POST":
		var body struct {
			Name string `json:"name"`
		}
		if err := bindJSON(r, &body); err != nil || body.Name == "" {
			fail(w, 400, "请填写学校名称")
			return
		}
		id, err := store.CreateSchool(body.Name)
		if err != nil {
			fail(w, 500, "创建学校失败: "+err.Error())
			return
		}
		if err := store.SwitchSchool(id); err != nil {
			fail(w, 500, "切换到新学校失败: "+err.Error())
			return
		}
		AddLog(u, "create", "新建学校 "+body.Name)
		okMsg(w, "学校已创建并切换成功", map[string]interface{}{"id": id})
	}
}

// handleSchoolUpdate PUT 改名 / DELETE 删除
func handleSchoolUpdate(w http.ResponseWriter, r *http.Request) {
	u := requireLogin(w, r)
	if u == nil {
		return
	}
	id, okID := pathID(r, "id")
	if !okID {
		fail(w, 400, "参数错误")
		return
	}
	switch r.Method {
	case "PUT":
		var body struct {
			Name string `json:"name"`
		}
		if err := bindJSON(r, &body); err != nil || body.Name == "" {
			fail(w, 400, "请填写学校名称")
			return
		}
		if err := store.RenameSchool(id, body.Name); err != nil {
			fail(w, 500, err.Error())
			return
		}
		AddLog(u, "update", "修改学校 #"+itoa(id))
		okMsg(w, "已保存", nil)
	case "DELETE":
		if err := store.DeleteSchool(id); err != nil {
			fail(w, 400, err.Error())
			return
		}
		AddLog(u, "delete", "删除学校 #"+itoa(id))
		okMsg(w, "已删除", nil)
	}
}

// handleSchoolSwitch 切换当前学校
func handleSchoolSwitch(w http.ResponseWriter, r *http.Request) {
	u := requireLogin(w, r)
	if u == nil {
		return
	}
	id, okID := pathID(r, "id")
	if !okID {
		fail(w, 400, "参数错误")
		return
	}
	if err := store.SwitchSchool(id); err != nil {
		fail(w, 500, "切换失败: "+err.Error())
		return
	}
	AddLog(u, "update", "切换到学校 "+store.CurrentSchoolName)
	okMsg(w, "已切换到 "+store.CurrentSchoolName, map[string]interface{}{
		"id": store.CurrentSchoolID, "name": store.CurrentSchoolName,
	})
}
