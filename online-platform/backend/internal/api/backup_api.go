package api

import (
	"net/http"
	"path/filepath"

	"school-scheduler/internal/backup"
)

func handleBackupList(w http.ResponseWriter, r *http.Request) {
	if requirePerm(w, r, "backup") == nil {
		return
	}
	ok(w, backup.List())
}

func handleBackupCreate(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "backup")
	if u == nil {
		return
	}
	name, err := backup.Create("manual")
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	AddLog(u, "backup", "手动备份 "+name)
	okMsg(w, "备份成功："+name, nil)
}

func handleBackupRestore(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "backup")
	if u == nil {
		return
	}
	id, okID := pathID(r, "id")
	if !okID {
		fail(w, 400, "参数错误")
		return
	}
	prevName, err := backup.Restore(id)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	AddLog(u, "restore", "数据恢复完成（当前数据已自动备份为 "+prevName+"）")
	okMsg(w, "恢复成功，当前数据已自动备份为 "+prevName+"。请重新登录。", nil)
}

func handleBackupDelete(w http.ResponseWriter, r *http.Request) {
	u := requirePerm(w, r, "backup")
	if u == nil {
		return
	}
	id, okID := pathID(r, "id")
	if !okID {
		fail(w, 400, "参数错误")
		return
	}
	if err := backup.Delete(id); err != nil {
		fail(w, 500, "删除失败")
		return
	}
	AddLog(u, "delete", "删除备份 #"+itoa(id))
	okMsg(w, "删除成功", nil)
}

func handleBackupDownload(w http.ResponseWriter, r *http.Request) {
	if requirePerm(w, r, "backup") == nil {
		return
	}
	id, okID := pathID(r, "id")
	if !okID {
		fail(w, 400, "参数错误")
		return
	}
	p, err := backup.GetFile(id)
	if err != nil {
		fail(w, 404, err.Error())
		return
	}
	setDownloadHeader(w, "application/octet-stream", filepath.Base(p))
	http.ServeFile(w, r, p)
}
