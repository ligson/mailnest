package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

func (a *App) handleListThreads(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("pageSize"), 20)
	accountID, _ := strconv.ParseInt(r.URL.Query().Get("accountId"), 10, 64)
	folderID, _ := strconv.ParseInt(r.URL.Query().Get("folderId"), 10, 64)
	items, total, err := a.store.ListMailThreads(storage.ListMailThreadsQuery{
		UserID:         userID,
		AccountID:      accountID,
		FolderID:       folderID,
		SystemFolder:   r.URL.Query().Get("systemFolder"),
		Keyword:        r.URL.Query().Get("keyword"),
		From:           r.URL.Query().Get("from"),
		Subject:        r.URL.Query().Get("subject"),
		Body:           r.URL.Query().Get("body"),
		DateFrom:       parseDateQuery(r.URL.Query().Get("dateFrom")),
		DateTo:         parseDateQuery(r.URL.Query().Get("dateTo")),
		HasAttachments: parseBoolQuery(r.URL.Query().Get("hasAttachments")),
		IsRead:         parseBoolQuery(r.URL.Query().Get("isRead")),
		Starred:        parseBoolQuery(r.URL.Query().Get("starred")),
		Limit:          pageSize,
		Offset:         (page - 1) * pageSize,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取会话列表失败")
		return
	}
	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload = append(payload, mailThreadPayload(item))
	}
	response.OK(w, "获取成功", map[string]any{
		"items":    payload,
		"page":     page,
		"pageSize": pageSize,
		"total":    total,
	})
}

func (a *App) handleThreadDetail(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	threadID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || threadID <= 0 {
		response.Error(w, http.StatusBadRequest, "会话 ID 格式错误")
		return
	}
	thread, err := a.store.FindMailThreadByID(userID, threadID)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "会话不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取会话失败")
		return
	}
	messages, err := a.store.ListMailThreadMessages(userID, threadID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取会话邮件失败")
		return
	}
	response.OK(w, "获取成功", mailThreadDetailPayload(thread, messages))
}

func (a *App) handleRebuildThreads(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	var req rebuildThreadsRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	accountID, ok := parseOptionalID(w, req.AccountID, "邮箱账号 ID")
	if !ok {
		return
	}
	scope := strings.ToLower(strings.TrimSpace(req.Scope))
	if scope != "all" {
		scope = "empty"
	}
	result, err := a.mailService.RebuildThreads(userID, accountID, scope)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "重建会话失败")
		return
	}
	response.OK(w, "重建完成", map[string]any{
		"processedCount": result.ProcessedCount,
		"threadCount":    result.ThreadCount,
	})
}

func (a *App) handleListRuleLogs(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("pageSize"), 20)
	messageID, _ := strconv.ParseInt(r.URL.Query().Get("messageId"), 10, 64)
	ruleID, _ := strconv.ParseInt(r.URL.Query().Get("ruleId"), 10, 64)
	logs, total, err := a.store.ListMailRuleLogs(storage.ListMailRuleLogsQuery{
		UserID:       userID,
		MessageID:    messageID,
		RuleID:       ruleID,
		ResultStatus: r.URL.Query().Get("resultStatus"),
		TriggerType:  r.URL.Query().Get("triggerType"),
		Limit:        pageSize,
		Offset:       (page - 1) * pageSize,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取规则记录失败")
		return
	}
	items := make([]map[string]any, 0, len(logs))
	for _, item := range logs {
		items = append(items, mailRuleLogPayload(item))
	}
	response.OK(w, "获取成功", map[string]any{
		"items":    items,
		"page":     page,
		"pageSize": pageSize,
		"total":    total,
	})
}

func (a *App) handleMessageRuleLogs(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	messageID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || messageID <= 0 {
		response.Error(w, http.StatusBadRequest, "邮件 ID 格式错误")
		return
	}
	if _, err := a.store.FindMailMessageByID(userID, messageID); errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮件不存在")
		return
	} else if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取邮件失败")
		return
	}
	logs, total, err := a.store.ListMailRuleLogs(storage.ListMailRuleLogsQuery{
		UserID:    userID,
		MessageID: messageID,
		Limit:     50,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取规则记录失败")
		return
	}
	items := make([]map[string]any, 0, len(logs))
	for _, item := range logs {
		items = append(items, mailRuleLogPayload(item))
	}
	response.OK(w, "获取成功", map[string]any{
		"items":    items,
		"page":     1,
		"pageSize": 50,
		"total":    total,
	})
}
