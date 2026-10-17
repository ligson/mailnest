package api

import (
	"errors"
	"log"
	"net/http"

	"mailnest-be/internal/response"
	"mailnest-be/internal/searchindex"
)

func (a *App) handleSearchStatus(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUserID(r)
	if a.searchIndex == nil {
		response.OK(w, "", searchindex.Status{Engine: "database", Mode: "database", Phase: "disabled", Message: a.searchStartupError})
		return
	}
	user, err := a.store.FindUserByID(userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "读取检索状态失败")
		return
	}
	status, err := a.searchIndex.Status(r.Context(), userID, user.IsAdmin)
	if err != nil {
		log.Printf("读取检索状态失败 user_id=%d err=%v", userID, err)
		response.Error(w, http.StatusInternalServerError, "读取检索状态失败")
		return
	}
	response.OK(w, "", status)
}
func (a *App) handleSearchRebuild(w http.ResponseWriter, r *http.Request) {
	if a.searchIndex == nil {
		response.Error(w, http.StatusConflict, "Bleve 检索未启用或无法启动，请先检查后端配置")
		return
	}
	if err := a.searchIndex.RequestRebuild(); err != nil {
		if errors.Is(err, searchindex.ErrBusy) {
			response.Error(w, http.StatusConflict, err.Error())
			return
		}
		response.Error(w, http.StatusServiceUnavailable, "索引服务暂不可用")
		return
	}
	response.JSON(w, http.StatusAccepted, true, "已安排后台重建，现有邮件不受影响", map[string]any{})
}
