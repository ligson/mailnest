package api

import (
	"errors"
	"net/http"
	"strconv"

	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

func (a *App) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	summaries, err := a.store.ListAdminUserSummaries()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取用户列表失败")
		return
	}
	items := make([]map[string]any, 0, len(summaries))
	for _, summary := range summaries {
		items = append(items, adminUserSummaryPayload(summary))
	}
	response.OK(w, "获取成功", map[string]any{"items": items})
}

func (a *App) handleAdminUpdateUserEnabled(w http.ResponseWriter, r *http.Request) {
	currentUserID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}

	userID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.Error(w, http.StatusBadRequest, "用户 ID 格式错误")
		return
	}

	var req updateUserEnabledRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	if userID == currentUserID && !req.Enabled {
		response.Error(w, http.StatusBadRequest, "不能停用当前登录的管理员账号")
		return
	}

	user, err := a.store.UpdateUserEnabled(userID, req.Enabled)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "用户不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "更新用户状态失败")
		return
	}
	response.OK(w, "更新成功", adminUserSummaryPayload(storage.AdminUserSummary{User: user}))
}
