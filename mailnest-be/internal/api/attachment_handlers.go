package api

import (
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
)

func (a *App) handleListAttachments(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("pageSize"), 20)
	accountID, _ := strconv.ParseInt(r.URL.Query().Get("accountId"), 10, 64)
	folderID, _ := strconv.ParseInt(r.URL.Query().Get("folderId"), 10, 64)
	items, total, err := a.store.ListAttachments(storage.ListAttachmentsQuery{
		UserID:      userID,
		Keyword:     r.URL.Query().Get("keyword"),
		ContentType: r.URL.Query().Get("contentType"),
		AccountID:   accountID,
		FolderID:    folderID,
		Inline:      parseBoolQuery(r.URL.Query().Get("inline")),
		DateFrom:    parseDateQuery(r.URL.Query().Get("dateFrom")),
		DateTo:      parseDateQuery(r.URL.Query().Get("dateTo")),
		Limit:       pageSize,
		Offset:      (page - 1) * pageSize,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取附件失败")
		return
	}
	payloadItems := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloadItems = append(payloadItems, attachmentCenterPayload(item))
	}
	response.OK(w, "获取成功", map[string]any{
		"items":    payloadItems,
		"page":     page,
		"pageSize": pageSize,
		"total":    total,
	})
}

func (a *App) handleAttachmentContent(w http.ResponseWriter, r *http.Request) {
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
	attachmentID, err := strconv.ParseInt(r.PathValue("attachmentId"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "附件 ID 格式错误")
		return
	}

	if _, err := a.store.FindMailMessageByID(userID, messageID); errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "邮件不存在")
		return
	} else if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取邮件失败")
		return
	}

	attachment, err := a.store.FindMailAttachmentByID(userID, messageID, attachmentID)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "附件不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取附件失败")
		return
	}

	contentType := "application/octet-stream"
	if attachment.ContentType.Valid && strings.TrimSpace(attachment.ContentType.String) != "" {
		contentType = attachment.ContentType.String
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": attachment.Filename}))
	http.ServeFile(w, r, attachment.FilePath)
}

func (a *App) handleInlineAttachmentContent(w http.ResponseWriter, r *http.Request) {
	messageID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "邮件 ID 格式错误")
		return
	}
	attachmentID, err := strconv.ParseInt(r.PathValue("attachmentId"), 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "附件 ID 格式错误")
		return
	}
	userID, err := strconv.ParseInt(r.URL.Query().Get("uid"), 10, 64)
	if err != nil || userID <= 0 {
		response.Error(w, http.StatusUnauthorized, "内嵌图片链接已失效")
		return
	}
	expiresAt, err := strconv.ParseInt(r.URL.Query().Get("exp"), 10, 64)
	if err != nil || time.Now().Unix() > expiresAt {
		response.Error(w, http.StatusUnauthorized, "内嵌图片链接已失效")
		return
	}
	signature := r.URL.Query().Get("sig")
	if !validInlineAttachmentSignature(a.cfg.App.JWTSecret, userID, messageID, attachmentID, expiresAt, signature) {
		response.Error(w, http.StatusUnauthorized, "内嵌图片链接已失效")
		return
	}

	attachment, err := a.store.FindMailAttachmentByID(userID, messageID, attachmentID)
	if errors.Is(err, storage.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "附件不存在")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "获取附件失败")
		return
	}

	contentType := "application/octet-stream"
	if attachment.ContentType.Valid && strings.TrimSpace(attachment.ContentType.String) != "" {
		contentType = attachment.ContentType.String
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": attachment.Filename}))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, attachment.FilePath)
}
