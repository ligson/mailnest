package api

import (
	"errors"
	"net/http"
	"strconv"

	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

func (a *App) handleListSendLogs(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("pageSize"), 20)
	accountID, _ := strconv.ParseInt(r.URL.Query().Get("accountId"), 10, 64)
	messageID, _ := strconv.ParseInt(r.URL.Query().Get("messageId"), 10, 64)
	logs, total, err := a.store.ListMailSendLogs(storage.ListMailSendLogsQuery{
		UserID:      userID,
		AccountID:   accountID,
		MessageID:   messageID,
		Status:      r.URL.Query().Get("status"),
		RetryStatus: r.URL.Query().Get("retryStatus"),
		ComposeMode: r.URL.Query().Get("composeMode"),
		Keyword:     r.URL.Query().Get("keyword"),
		DateFrom:    parseDateQuery(r.URL.Query().Get("dateFrom")),
		DateTo:      parseDateQuery(r.URL.Query().Get("dateTo")),
		Limit:       pageSize,
		Offset:      (page - 1) * pageSize,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取发送记录失败")
		return
	}
	items := make([]map[string]any, 0, len(logs))
	for _, item := range logs {
		items = append(items, mailSendLogPayload(item))
	}
	response.OK(w, "获取成功", map[string]any{
		"items":    items,
		"page":     page,
		"pageSize": pageSize,
		"total":    total,
	})
}

func (a *App) handleSendLogDetail(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "发送记录 ID 格式错误")
		return
	}
	item, err := a.store.FindMailSendLogByID(userID, id)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "发送记录不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取发送记录失败")
		return
	}
	response.OK(w, "获取成功", mailSendLogPayload(item))
}
