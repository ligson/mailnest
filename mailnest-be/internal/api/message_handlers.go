package api

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mailnest-be/internal/mail"
	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

func (a *App) handleListMessages(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	started := time.Now()

	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("pageSize"), 20)
	accountID, _ := strconv.ParseInt(r.URL.Query().Get("accountId"), 10, 64)
	folderID, _ := strconv.ParseInt(r.URL.Query().Get("folderId"), 10, 64)
	offset := (page - 1) * pageSize

	messages, total, err := a.store.ListMailMessagesByQuery(storage.ListMailMessagesQuery{
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
		Offset:         offset,
		SummaryOnly:    true,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取邮件列表失败")
		return
	}
	queryDuration := time.Since(started)

	items := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		items = append(items, messageListPayload(message))
	}

	response.OK(w, "获取成功", map[string]any{
		"items":    items,
		"page":     page,
		"pageSize": pageSize,
		"total":    total,
	})
	logSlowAPI("messages.list", started, "query="+queryDuration.String(), "items="+strconv.Itoa(len(items)), "total="+strconv.Itoa(total))
}

func (a *App) handleMessageBatchAction(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	var req messageBatchActionRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	messageIDs, ok := parseIDStrings(req.MessageIDs)
	if !ok || len(messageIDs) == 0 {
		response.Error(w, http.StatusBadRequest, "请选择邮件")
		return
	}
	folderID, ok := parseOptionalID(w, req.FolderID, "文件夹 ID")
	if !ok {
		return
	}
	result, err := a.store.BatchUpdateMailMessageStates(userID, messageIDs, strings.TrimSpace(req.Action), sqlNullInt64(folderID))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "批量操作失败："+err.Error())
		return
	}
	response.OK(w, "操作成功", map[string]any{
		"matchedCount": result.MatchedCount,
		"changedCount": result.ChangedCount,
		"skippedCount": result.SkippedCount,
	})
}

func (a *App) handleMessageBatchPreview(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	var req messageBatchPreviewRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	messageIDs, ok := parseIDStrings(req.MessageIDs)
	if !ok {
		response.Error(w, http.StatusBadRequest, "邮件 ID 格式错误")
		return
	}
	preview, err := a.store.PreviewMailMessageBatch(userID, messageIDs)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取批量预览失败")
		return
	}
	folderCounts := make([]map[string]any, 0, len(preview.FolderCounts))
	for _, item := range preview.FolderCounts {
		folderCounts = append(folderCounts, map[string]any{
			"folderId": strconv.FormatInt(item.FolderID, 10),
			"name":     item.Name,
			"count":    item.Count,
		})
	}
	response.OK(w, "获取成功", map[string]any{
		"total":        preview.Total,
		"readCount":    preview.ReadCount,
		"unreadCount":  preview.UnreadCount,
		"starredCount": preview.StarredCount,
		"spamCount":    preview.SpamCount,
		"deletedCount": preview.DeletedCount,
		"folderCounts": folderCounts,
	})
}

func (a *App) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	req, attachments, err := decodeSendMessageRequest(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	accountID, err := strconv.ParseInt(strings.TrimSpace(req.AccountID), 10, 64)
	if err != nil || accountID <= 0 {
		response.Error(w, http.StatusBadRequest, "请选择发件邮箱账号")
		return
	}
	to, ok := normalizeOutgoingAddresses(w, req.To, "收件人")
	if !ok {
		return
	}
	cc, ok := normalizeOutgoingAddresses(w, req.CC, "抄送人")
	if !ok {
		return
	}
	bcc, ok := normalizeOutgoingAddresses(w, req.BCC, "密送人")
	if !ok {
		return
	}
	if len(to) == 0 && len(cc) == 0 && len(bcc) == 0 {
		response.Error(w, http.StatusBadRequest, "至少需要填写一个收件人")
		return
	}
	req.Subject = strings.TrimSpace(req.Subject)
	req.TextBody = strings.TrimSpace(req.TextBody)
	req.HTMLBody = strings.TrimSpace(req.HTMLBody)
	if req.Subject == "" && req.TextBody == "" && req.HTMLBody == "" && len(attachments) == 0 && len(req.ForwardAttachmentIDs) == 0 {
		response.Error(w, http.StatusBadRequest, "主题和正文不能同时为空")
		return
	}
	if len([]rune(req.Subject)) > 500 {
		response.Error(w, http.StatusBadRequest, "邮件主题不能超过 500 个字符")
		return
	}
	sourceMessageID, ok := parseOptionalID(w, req.SourceMessageID, "来源邮件 ID")
	if !ok {
		return
	}
	draftID, ok := parseOptionalID(w, req.DraftID, "草稿 ID")
	if !ok {
		return
	}
	forwardAttachmentIDs, ok := parseOptionalIDs(w, req.ForwardAttachmentIDs, "转发附件 ID")
	if !ok {
		return
	}

	result, err := a.mailService.SendMessageWithLog(userID, accountID, mail.OutgoingMessage{
		DraftID:              draftID,
		To:                   to,
		CC:                   cc,
		BCC:                  bcc,
		Subject:              req.Subject,
		TextBody:             req.TextBody,
		HTMLBody:             req.HTMLBody,
		ComposeMode:          req.ComposeMode,
		SourceMessageID:      sourceMessageID,
		ForwardAttachmentIDs: forwardAttachmentIDs,
		Attachments:          attachments,
	})
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮箱账号不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusBadRequest, "发送失败："+err.Error())
		return
	}
	if draftID > 0 {
		if err := a.store.DeleteMailDraft(userID, draftID); err != nil && !errors.Is(err, storage.ErrNotFound) {
			response.Error(w, http.StatusInternalServerError, "邮件已发送，但删除草稿失败")
			return
		}
	}
	payload := messageListPayload(result.Message)
	payload["sendLog"] = mailSendLogPayload(result.Log)
	response.OK(w, "发送成功", payload)
}

func (a *App) handleMessageComposeContext(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "邮件 ID 格式错误")
		return
	}
	ctx, err := a.mailService.GetComposeContext(userID, id, r.URL.Query().Get("mode"))
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮件不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取写信上下文失败")
		return
	}
	response.OK(w, "获取成功", composeContextPayload(ctx))
}

func (a *App) handleMessageDetail(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	started := time.Now()

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "邮件 ID 格式错误")
		return
	}

	segmentStarted := time.Now()
	message, err := a.store.FindMailMessageByID(userID, id)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮件不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取邮件详情失败")
		return
	}
	if !message.IsRead {
		message.IsRead = true
		a.markMessageReadAsync(userID, id)
	}
	messageDuration := time.Since(segmentStarted)

	payload := messageListPayload(message)
	segmentStarted = time.Now()
	attachments, err := a.store.ListMailAttachments(userID, message.ID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取邮件附件失败")
		return
	}
	attachmentsDuration := time.Since(segmentStarted)
	segmentStarted = time.Now()
	htmlBody := readOptionalFile(message.HTMLBodyPath)
	htmlReadDuration := time.Since(segmentStarted)
	segmentStarted = time.Now()
	inlineContentIDs := referencedInlineContentIDs(htmlBody, attachments)
	payload["textBody"] = readOptionalFile(message.TextBodyPath)
	bodyReadDuration := time.Since(segmentStarted)
	segmentStarted = time.Now()
	payload["htmlBody"] = rewriteInlineCIDImages(htmlBody, attachments, inlineContentIDs, userID, message.ID, a.cfg.App.JWTSecret)
	rewriteDuration := time.Since(segmentStarted)
	payload["cc"] = splitAddressField(message.CCAddrs)
	payload["folder"] = message.Folder
	payload["messageId"] = nullableString(message.MessageID)
	payload["attachments"] = attachmentPayloads(message.ID, attachments, inlineContentIDs)

	response.OK(w, "获取成功", payload)
	logSlowAPI(
		"messages.detail",
		started,
		"id="+strconv.FormatInt(id, 10),
		"message="+messageDuration.String(),
		"attachments="+attachmentsDuration.String(),
		"htmlRead="+htmlReadDuration.String(),
		"bodyRead="+bodyReadDuration.String(),
		"cidRewrite="+rewriteDuration.String(),
		"attachmentCount="+strconv.Itoa(len(attachments)),
		"htmlBytes="+strconv.Itoa(len(htmlBody)),
	)
}

func (a *App) handleAssignMessageFolder(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	messageID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "邮件 ID 格式错误")
		return
	}
	var req assignMessageFolderRequest
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	var folderID sql.NullInt64
	if strings.TrimSpace(req.FolderID) != "" {
		parsedFolderID, err := strconv.ParseInt(req.FolderID, 10, 64)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "文件夹 ID 格式错误")
			return
		}
		if _, err := a.store.FindMailFolderByID(userID, parsedFolderID); errors.Is(err, storage.ErrNotFound) {
			response.Error(w, http.StatusNotFound, "文件夹不存在")
			return
		} else if err != nil {
			response.Error(w, http.StatusInternalServerError, "获取文件夹失败")
			return
		}
		folderID = sql.NullInt64{Int64: parsedFolderID, Valid: true}
	}

	if err := a.store.UpdateMailMessageFolder(userID, messageID, folderID); errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮件不存在")
		return
	} else if err != nil {
		response.Error(w, http.StatusInternalServerError, "更新邮件文件夹失败")
		return
	}
	response.OK(w, "更新成功", nil)
}

func (a *App) markMessageReadAsync(userID, messageID int64) {
	go func() {
		isRead := true
		if err := a.store.UpsertMailMessageState(userID, messageID, &isRead, nil, nil, nil); err != nil {
			log.Printf("标记邮件已读失败 userID=%d messageID=%d err=%v", userID, messageID, err)
		}
	}()
}
