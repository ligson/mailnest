package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"mailnest-be/internal/mail"
	"mailnest-be/internal/storage"
)

func userPayload(user storage.User) map[string]any {
	return map[string]any{
		"id":        strconv.FormatInt(user.ID, 10),
		"username":  user.Username,
		"email":     user.Email,
		"nickname":  nullableString(user.Nickname),
		"bio":       nullableString(user.Bio),
		"uiTheme":   normalizeUITheme(user.UITheme),
		"isAdmin":   user.IsAdmin,
		"enabled":   user.Enabled,
		"avatarUrl": profileAvatarURL(user),
	}
}

func adminUserSummaryPayload(summary storage.AdminUserSummary) map[string]any {
	return map[string]any{
		"id":               strconv.FormatInt(summary.User.ID, 10),
		"username":         summary.User.Username,
		"email":            summary.User.Email,
		"nickname":         nullableString(summary.User.Nickname),
		"isAdmin":          summary.User.IsAdmin,
		"enabled":          summary.User.Enabled,
		"mailAccountCount": summary.MailAccountCount,
		"messageCount":     summary.MessageCount,
		"attachmentCount":  summary.AttachmentCount,
		"attachmentBytes":  summary.AttachmentBytes,
		"contactCount":     summary.ContactCount,
		"folderCount":      summary.FolderCount,
		"ruleCount":        summary.RuleCount,
		"lastMessageAt":    nullableTime(summary.LastMessageAt),
		"lastSyncAt":       nullableTime(summary.LastSyncAt),
		"createdAt":        summary.User.CreatedAt,
		"updatedAt":        summary.User.UpdatedAt,
	}
}

func profileAvatarURL(user storage.User) any {
	if !user.AvatarPath.Valid || strings.TrimSpace(user.AvatarPath.String) == "" {
		return nil
	}
	return "/api/v1/profile/avatar/content"
}

func mailAccountPayload(account storage.MailAccount) map[string]any {
	var lastSyncAt any
	if account.LastSyncAt.Valid {
		lastSyncAt = account.LastSyncAt.Time
	}

	var lastSyncStatus any
	if account.LastSyncStatus.Valid {
		lastSyncStatus = account.LastSyncStatus.String
	}

	var lastSyncError any
	if account.LastSyncError.Valid {
		lastSyncError = account.LastSyncError.String
	}

	return map[string]any{
		"id":                   strconv.FormatInt(account.ID, 10),
		"provider":             account.Provider,
		"authType":             account.AuthType,
		"displayName":          account.DisplayName,
		"email":                account.Email,
		"imapHost":             account.IMAPHost,
		"imapPort":             account.IMAPPort,
		"imapTls":              account.IMAPTLS,
		"imapUsername":         account.IMAPUsername,
		"smtpHost":             account.SMTPHost,
		"smtpPort":             account.SMTPPort,
		"smtpTls":              account.SMTPTLS,
		"smtpStartTls":         account.SMTPStartTLS,
		"smtpUsername":         account.SMTPUsername,
		"smtpConfigured":       strings.TrimSpace(account.SMTPHost) != "",
		"sentFolder":           normalizeSentFolder(account.SentFolder),
		"signatureHtml":        account.SignatureHTML,
		"pollIntervalMinutes":  account.PollIntervalMinutes,
		"enabled":              account.Enabled,
		"lastSyncAt":           lastSyncAt,
		"lastSyncStatus":       lastSyncStatus,
		"lastSyncError":        lastSyncError,
		"fullSyncStatus":       account.FullSyncStatus,
		"fullSyncTotal":        account.FullSyncTotal,
		"fullSyncProcessed":    account.FullSyncProcessed,
		"fullSyncNewCount":     account.FullSyncNewCount,
		"fullSyncStartedAt":    nullableTime(account.FullSyncStartedAt),
		"fullSyncFinishedAt":   nullableTime(account.FullSyncFinishedAt),
		"fullSyncError":        nullableString(account.FullSyncError),
		"cleanupEnabled":       account.CleanupEnabled,
		"cleanupRetentionDays": account.CleanupRetentionDays,
	}
}

func fullSyncStatusPayload(status mail.FullSyncStatus) map[string]any {
	return map[string]any{
		"fullSyncStatus":       status.Status,
		"fullSyncTotal":        status.Total,
		"fullSyncProcessed":    status.Processed,
		"fullSyncNewCount":     status.NewCount,
		"fullSyncStartedAt":    nullableTime(status.StartedAt),
		"fullSyncFinishedAt":   nullableTime(status.FinishedAt),
		"fullSyncError":        nullableString(status.Error),
		"cleanupEnabled":       status.CleanupEnabled,
		"cleanupRetentionDays": status.RetentionDays,
	}
}

func composeContextPayload(ctx mail.ComposeContext) map[string]any {
	attachments := make([]map[string]any, 0, len(ctx.ForwardAttachments))
	for _, attachment := range ctx.ForwardAttachments {
		attachments = append(attachments, map[string]any{
			"id":          strconv.FormatInt(attachment.ID, 10),
			"filename":    attachment.Filename,
			"contentType": attachment.ContentType,
			"size":        attachment.Size,
			"selected":    attachment.Selected,
		})
	}
	return map[string]any{
		"mode":               ctx.Mode,
		"sourceMessageId":    strconv.FormatInt(ctx.SourceMessageID, 10),
		"accountId":          strconv.FormatInt(ctx.AccountID, 10),
		"to":                 ctx.To,
		"cc":                 ctx.CC,
		"bcc":                ctx.BCC,
		"subject":            ctx.Subject,
		"textBody":           ctx.TextBody,
		"htmlBody":           ctx.HTMLBody,
		"forwardAttachments": attachments,
	}
}

func messageListPayload(message storage.MailMessage) map[string]any {
	return map[string]any{
		"id":             strconv.FormatInt(message.ID, 10),
		"accountId":      strconv.FormatInt(message.AccountID, 10),
		"localFolderId":  nullableInt64(message.LocalFolderID),
		"subject":        nullableString(message.Subject),
		"from":           nullableString(message.FromAddr),
		"to":             splitAddressField(message.ToAddrs),
		"sentAt":         nullableTime(message.SentAt),
		"receivedAt":     nullableTime(message.ReceivedAt),
		"hasAttachments": message.HasAttachments,
		"isRead":         message.IsRead,
		"starred":        message.Starred,
		"isSpam":         message.IsSpam,
		"spamAt":         nullableTime(message.SpamAt),
		"deletedAt":      nullableTime(message.DeletedAt),
	}
}

func mailFolderPayload(folder storage.MailFolder) map[string]any {
	return map[string]any{
		"id":        strconv.FormatInt(folder.ID, 10),
		"name":      folder.Name,
		"color":     nullableString(folder.Color),
		"sortOrder": folder.SortOrder,
		"ruleCount": folder.RuleCount,
	}
}

func contactPayload(contact storage.Contact) map[string]any {
	displayName := nullableString(contact.DisplayName)
	nickname := nullableString(contact.Nickname)
	preferredName := contact.Email
	if name, ok := nickname.(string); ok && strings.TrimSpace(name) != "" {
		preferredName = name
	} else if name, ok := displayName.(string); ok && strings.TrimSpace(name) != "" {
		preferredName = name
	}
	return map[string]any{
		"id":          strconv.FormatInt(contact.ID, 10),
		"email":       contact.Email,
		"displayName": displayName,
		"nickname":    nickname,
		"name":        preferredName,
		"phone":       nullableString(contact.Phone),
		"company":     nullableString(contact.Company),
		"notes":       nullableString(contact.Notes),
		"source":      contact.Source,
		"firstSeenAt": nullableTime(contact.FirstSeenAt),
		"lastSeenAt":  nullableTime(contact.LastSeenAt),
		"createdAt":   contact.CreatedAt,
		"updatedAt":   contact.UpdatedAt,
	}
}

func mailRulePayload(rule storage.MailRule) map[string]any {
	conditions := make([]map[string]any, 0, len(rule.Conditions))
	for _, condition := range rule.Conditions {
		conditions = append(conditions, map[string]any{
			"id":       strconv.FormatInt(condition.ID, 10),
			"field":    condition.Field,
			"operator": condition.Operator,
			"value":    condition.Value,
		})
	}
	return map[string]any{
		"id":             strconv.FormatInt(rule.ID, 10),
		"name":           rule.Name,
		"enabled":        rule.Enabled,
		"matchMode":      rule.MatchMode,
		"priority":       rule.Priority,
		"stopOnMatch":    rule.StopOnMatch,
		"actionType":     rule.ActionType,
		"targetFolderId": nullableRuleTargetFolderID(rule.TargetFolderID),
		"sortOrder":      rule.SortOrder,
		"conditions":     conditions,
	}
}

func nullableRuleTargetFolderID(id int64) any {
	if id <= 0 {
		return nil
	}
	return strconv.FormatInt(id, 10)
}

func nullableString(value sql.NullString) any {
	if value.Valid {
		return value.String
	}
	return nil
}

func nullableTime(value sql.NullTime) any {
	if value.Valid {
		return value.Time
	}
	return nil
}

func nullableInt64(value sql.NullInt64) any {
	if value.Valid {
		return strconv.FormatInt(value.Int64, 10)
	}
	return nil
}

func splitAddressField(value sql.NullString) []string {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return []string{}
	}
	parts := strings.Split(value.String, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func attachmentCenterPayload(item storage.AttachmentListItem) map[string]any {
	return map[string]any{
		"id":             strconv.FormatInt(item.Attachment.ID, 10),
		"messageId":      strconv.FormatInt(item.Attachment.MessageID, 10),
		"filename":       item.Attachment.Filename,
		"contentType":    nullableString(item.Attachment.ContentType),
		"contentId":      nullableString(item.Attachment.ContentID),
		"inline":         item.Attachment.Inline,
		"size":           item.Attachment.Size,
		"downloadUrl":    fmt.Sprintf("/api/v1/messages/%d/attachments/%d/content", item.Attachment.MessageID, item.Attachment.ID),
		"accountId":      strconv.FormatInt(item.AccountID, 10),
		"folderId":       nullableInt64(item.LocalFolderID),
		"messageSubject": nullableString(item.MessageSubject),
		"messageFrom":    nullableString(item.MessageFrom),
		"messageTime":    nullableTime(item.MessageTime),
	}
}

func syncJobPayload(job storage.MailSyncJob) map[string]any {
	return map[string]any{
		"id":              strconv.FormatInt(job.ID, 10),
		"accountId":       strconv.FormatInt(job.AccountID, 10),
		"triggerType":     job.TriggerType,
		"status":          job.Status,
		"startedAt":       nullableTime(job.StartedAt),
		"finishedAt":      nullableTime(job.FinishedAt),
		"newMessageCount": job.NewMessageCount,
		"errorMessage":    nullableString(job.ErrorMessage),
	}
}

func syncJobEventPayload(event storage.MailSyncJobEvent) map[string]any {
	var detail any
	if event.DetailJSON.Valid && strings.TrimSpace(event.DetailJSON.String) != "" {
		_ = json.Unmarshal([]byte(event.DetailJSON.String), &detail)
	}
	return map[string]any{
		"id":        strconv.FormatInt(event.ID, 10),
		"jobId":     strconv.FormatInt(event.JobID, 10),
		"level":     event.Level,
		"phase":     event.Phase,
		"message":   event.Message,
		"detail":    detail,
		"createdAt": event.CreatedAt,
	}
}
