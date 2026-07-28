package api

import (
	"net/http"
	"strconv"

	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

func (a *App) handleListServerDeleteLogs(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("pageSize"), 50)
	accountID, _ := strconv.ParseInt(r.URL.Query().Get("accountId"), 10, 64)
	messageID, _ := strconv.ParseInt(r.URL.Query().Get("messageId"), 10, 64)
	syncJobID, _ := strconv.ParseInt(r.URL.Query().Get("syncJobId"), 10, 64)
	logs, total, err := a.store.ListMailServerDeleteLogs(storage.ListMailServerDeleteLogsQuery{
		UserID:    userID,
		AccountID: accountID,
		MessageID: messageID,
		SyncJobID: syncJobID,
		Status:    r.URL.Query().Get("status"),
		Keyword:   r.URL.Query().Get("keyword"),
		DateFrom:  parseDateQuery(r.URL.Query().Get("dateFrom")),
		DateTo:    parseDateQuery(r.URL.Query().Get("dateTo")),
		Limit:     pageSize,
		Offset:    (page - 1) * pageSize,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取服务器删除日志失败")
		return
	}
	items := make([]map[string]any, 0, len(logs))
	for _, item := range logs {
		items = append(items, mailServerDeleteLogPayload(item))
	}
	response.OK(w, "获取成功", map[string]any{
		"items":    items,
		"page":     page,
		"pageSize": pageSize,
		"total":    total,
	})
}
