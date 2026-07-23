package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"mailnest-be/internal/mail"
	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

func (a *App) handleListDrafts(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("pageSize"), 20)
	drafts, total, err := a.store.ListMailDrafts(storage.ListMailDraftsQuery{
		UserID: userID,
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取草稿列表失败")
		return
	}
	items := make([]map[string]any, 0, len(drafts))
	for _, draft := range drafts {
		items = append(items, draftPayload(draft))
	}
	response.OK(w, "获取成功", map[string]any{
		"items":    items,
		"page":     page,
		"pageSize": pageSize,
		"total":    total,
	})
}

func (a *App) handleDraftDetail(w http.ResponseWriter, r *http.Request) {
	userID, draftID, ok := draftRouteIDs(w, r)
	if !ok {
		return
	}
	draft, err := a.store.FindMailDraftByID(userID, draftID)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "草稿不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取草稿失败")
		return
	}
	response.OK(w, "获取成功", draftPayload(draft))
}

func (a *App) handleCreateDraft(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	params, ok := decodeDraftParams(w, r, userID, 0)
	if !ok {
		return
	}
	draft, err := a.store.SaveMailDraft(params)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮箱账号或来源邮件不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusBadRequest, "保存草稿失败："+err.Error())
		return
	}
	response.OK(w, "草稿已保存", draftPayload(draft))
}

func (a *App) handleUpdateDraft(w http.ResponseWriter, r *http.Request) {
	userID, draftID, ok := draftRouteIDs(w, r)
	if !ok {
		return
	}
	params, ok := decodeDraftParams(w, r, userID, draftID)
	if !ok {
		return
	}
	draft, err := a.store.SaveMailDraft(params)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "草稿、邮箱账号或来源邮件不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusBadRequest, "保存草稿失败："+err.Error())
		return
	}
	response.OK(w, "草稿已保存", draftPayload(draft))
}

func (a *App) handleDeleteDraft(w http.ResponseWriter, r *http.Request) {
	userID, draftID, ok := draftRouteIDs(w, r)
	if !ok {
		return
	}
	if err := a.store.DeleteMailDraft(userID, draftID); errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "草稿不存在")
		return
	} else if err != nil {
		response.Error(w, http.StatusInternalServerError, "删除草稿失败")
		return
	}
	response.OK(w, "草稿已删除", nil)
}

func (a *App) handleSendDraft(w http.ResponseWriter, r *http.Request) {
	userID, draftID, ok := draftRouteIDs(w, r)
	if !ok {
		return
	}
	draft, err := a.store.FindMailDraftByID(userID, draftID)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "草稿不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取草稿失败")
		return
	}
	localNames := draftStringSlice(draft.LocalAttachmentNamesJSON)
	if len(localNames) > 0 {
		response.Error(w, http.StatusBadRequest, "草稿包含本地附件，请打开草稿后重新选择附件再发送")
		return
	}
	to, ok := normalizeOutgoingAddresses(w, draftStringSlice(draft.ToAddrsJSON), "收件人")
	if !ok {
		return
	}
	cc, ok := normalizeOutgoingAddresses(w, draftStringSlice(draft.CCAddrsJSON), "抄送人")
	if !ok {
		return
	}
	bcc, ok := normalizeOutgoingAddresses(w, draftStringSlice(draft.BCCAddrsJSON), "密送人")
	if !ok {
		return
	}
	if len(to) == 0 && len(cc) == 0 && len(bcc) == 0 {
		response.Error(w, http.StatusBadRequest, "至少需要填写一个收件人")
		return
	}
	forwardAttachmentIDs := make([]int64, 0)
	for _, value := range draftStringSlice(draft.ForwardAttachmentIDsJSON) {
		id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err == nil && id > 0 {
			forwardAttachmentIDs = append(forwardAttachmentIDs, id)
		}
	}
	if strings.TrimSpace(draft.Subject) == "" && strings.TrimSpace(draft.TextBody) == "" && strings.TrimSpace(draft.HTMLBody) == "" && len(forwardAttachmentIDs) == 0 {
		response.Error(w, http.StatusBadRequest, "主题和正文不能同时为空")
		return
	}
	sent, err := a.mailService.SendMessage(userID, draft.AccountID, mail.OutgoingMessage{
		To:                   to,
		CC:                   cc,
		BCC:                  bcc,
		Subject:              strings.TrimSpace(draft.Subject),
		TextBody:             draft.TextBody,
		HTMLBody:             draft.HTMLBody,
		ComposeMode:          normalizeDraftComposeMode(draft.ComposeMode),
		SourceMessageID:      nullableInt64OrZero(draft.SourceMessageID),
		ForwardAttachmentIDs: forwardAttachmentIDs,
	})
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮箱账号或来源邮件不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusBadRequest, "发送失败："+err.Error())
		return
	}
	if err := a.store.DeleteMailDraft(userID, draft.ID); err != nil && !errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusInternalServerError, "邮件已发送，但删除草稿失败")
		return
	}
	response.OK(w, "发送成功", messageListPayload(sent))
}

func draftRouteIDs(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return 0, 0, false
	}
	draftID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || draftID <= 0 {
		response.Error(w, http.StatusBadRequest, "草稿 ID 格式错误")
		return 0, 0, false
	}
	return userID, draftID, true
}

func decodeDraftParams(w http.ResponseWriter, r *http.Request, userID, draftID int64) (storage.SaveMailDraftParams, bool) {
	var req saveDraftRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return storage.SaveMailDraftParams{}, false
	}
	accountID, err := strconv.ParseInt(strings.TrimSpace(req.AccountID), 10, 64)
	if err != nil || accountID <= 0 {
		response.Error(w, http.StatusBadRequest, "请选择发件邮箱账号")
		return storage.SaveMailDraftParams{}, false
	}
	sourceMessageID, ok := parseOptionalID(w, req.SourceMessageID, "来源邮件 ID")
	if !ok {
		return storage.SaveMailDraftParams{}, false
	}
	to := nonEmptyStrings(req.To)
	cc := nonEmptyStrings(req.CC)
	bcc := nonEmptyStrings(req.BCC)
	if len([]rune(req.Subject)) > 500 {
		response.Error(w, http.StatusBadRequest, "邮件主题不能超过 500 个字符")
		return storage.SaveMailDraftParams{}, false
	}
	forwardAttachmentIDs, ok := parseOptionalIDs(w, req.ForwardAttachmentIDs, "转发附件 ID")
	if !ok {
		return storage.SaveMailDraftParams{}, false
	}
	forwardIDStrings := make([]string, 0, len(forwardAttachmentIDs))
	for _, id := range forwardAttachmentIDs {
		forwardIDStrings = append(forwardIDStrings, strconv.FormatInt(id, 10))
	}
	return storage.SaveMailDraftParams{
		ID:                       draftID,
		UserID:                   userID,
		AccountID:                accountID,
		ComposeMode:              normalizeDraftComposeMode(req.ComposeMode),
		SourceMessageID:          sql.NullInt64{Int64: sourceMessageID, Valid: sourceMessageID > 0},
		ToAddrsJSON:              mustJSONString(to),
		CCAddrsJSON:              mustJSONString(cc),
		BCCAddrsJSON:             mustJSONString(bcc),
		Subject:                  strings.TrimSpace(req.Subject),
		TextBody:                 req.TextBody,
		HTMLBody:                 req.HTMLBody,
		ForwardAttachmentIDsJSON: mustJSONString(forwardIDStrings),
		LocalAttachmentNamesJSON: mustJSONString(nonEmptyStrings(req.LocalAttachmentNames)),
	}, true
}

func draftPayload(draft storage.MailDraft) map[string]any {
	return map[string]any{
		"id":                   strconv.FormatInt(draft.ID, 10),
		"accountId":            strconv.FormatInt(draft.AccountID, 10),
		"composeMode":          normalizeDraftComposeMode(draft.ComposeMode),
		"sourceMessageId":      nullableInt64(draft.SourceMessageID),
		"to":                   draftStringSlice(draft.ToAddrsJSON),
		"cc":                   draftStringSlice(draft.CCAddrsJSON),
		"bcc":                  draftStringSlice(draft.BCCAddrsJSON),
		"subject":              draft.Subject,
		"textBody":             draft.TextBody,
		"htmlBody":             draft.HTMLBody,
		"forwardAttachmentIds": draftStringSlice(draft.ForwardAttachmentIDsJSON),
		"localAttachmentNames": draftStringSlice(draft.LocalAttachmentNamesJSON),
		"createdAt":            draft.CreatedAt,
		"updatedAt":            draft.UpdatedAt,
	}
}

func normalizeDraftComposeMode(value string) string {
	switch strings.TrimSpace(value) {
	case "reply", "replyAll", "forward":
		return strings.TrimSpace(value)
	default:
		return "new"
	}
}

func mustJSONString(values []string) string {
	data, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func draftStringSlice(value string) []string {
	var result []string
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return []string{}
	}
	return nonEmptyStrings(result)
}

func nonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func nullableInt64OrZero(value sql.NullInt64) int64 {
	if value.Valid {
		return value.Int64
	}
	return 0
}
