package storage

import (
	"database/sql"
	"errors"
	"strings"
)

func (s *Store) CreateMailSendLog(params CreateMailSendLogParams) (MailSendLog, error) {
	logID, err := s.db.insertAndGetID(
		`INSERT INTO mail_send_logs (
			user_id, account_id, draft_id, source_message_id, compose_mode, recipients_json, subject,
			attachment_count, status, retry_status, retry_count, started_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`,
		params.UserID,
		params.AccountID,
		nullInt64Value(params.DraftID),
		nullInt64Value(params.SourceMessageID),
		normalizeSendLogComposeMode(params.ComposeMode),
		strings.TrimSpace(params.RecipientsJSON),
		strings.TrimSpace(params.Subject),
		params.AttachmentCount,
		"sending",
		"none",
		nullTimeValue(params.StartedAt),
	)
	if err != nil {
		return MailSendLog{}, err
	}
	return s.FindMailSendLogByID(params.UserID, logID)
}

func (s *Store) UpdateMailSendLog(params UpdateMailSendLogParams) (MailSendLog, error) {
	result, err := s.db.Exec(
		`UPDATE mail_send_logs
		SET message_id = ?, smtp_message_id = ?, status = ?, retry_status = ?, retry_count = ?, error_message = ?,
			finished_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		nullInt64Value(params.MessageID),
		nullIfEmpty(params.SMTPMessageID),
		normalizeSendLogStatus(params.Status),
		normalizeSendLogRetryStatus(params.RetryStatus),
		params.RetryCount,
		nullIfEmpty(strings.TrimSpace(params.ErrorMessage)),
		nullTimeValue(params.FinishedAt),
		params.UserID,
		params.ID,
	)
	if err != nil {
		return MailSendLog{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return MailSendLog{}, err
	}
	if count == 0 {
		return MailSendLog{}, ErrNotFound
	}
	return s.FindMailSendLogByID(params.UserID, params.ID)
}

func (s *Store) FindMailSendLogByID(userID, id int64) (MailSendLog, error) {
	row := s.db.QueryRow(
		`SELECT l.id, l.user_id, l.account_id, a.email, l.message_id, m.subject, l.draft_id, l.source_message_id,
			l.compose_mode, l.smtp_message_id, l.recipients_json, l.subject, l.attachment_count, l.status,
			l.retry_status, l.retry_count, l.error_message, l.started_at, l.finished_at, l.created_at, l.updated_at
		FROM mail_send_logs l
		LEFT JOIN mail_accounts a ON a.user_id = l.user_id AND a.id = l.account_id
		LEFT JOIN mail_messages m ON m.user_id = l.user_id AND m.id = l.message_id
		WHERE l.user_id = ? AND l.id = ?`,
		userID,
		id,
	)
	return scanMailSendLog(row)
}

func (s *Store) ListMailSendLogs(query ListMailSendLogsQuery) ([]MailSendLog, int, error) {
	where := "WHERE l.user_id = ?"
	args := []any{query.UserID}
	if query.AccountID > 0 {
		where += " AND l.account_id = ?"
		args = append(args, query.AccountID)
	}
	if query.MessageID > 0 {
		where += " AND l.message_id = ?"
		args = append(args, query.MessageID)
	}
	if query.Status = strings.TrimSpace(query.Status); query.Status != "" {
		where += " AND l.status = ?"
		args = append(args, query.Status)
	}
	if query.RetryStatus = strings.TrimSpace(query.RetryStatus); query.RetryStatus != "" {
		where += " AND l.retry_status = ?"
		args = append(args, query.RetryStatus)
	}
	if query.ComposeMode = strings.TrimSpace(query.ComposeMode); query.ComposeMode != "" {
		where += " AND l.compose_mode = ?"
		args = append(args, query.ComposeMode)
	}
	if query.Keyword = strings.TrimSpace(query.Keyword); query.Keyword != "" {
		like := "%" + query.Keyword + "%"
		where += ` AND (
			COALESCE(l.subject, '') LIKE ?
			OR COALESCE(l.recipients_json, '') LIKE ?
			OR COALESCE(l.smtp_message_id, '') LIKE ?
			OR COALESCE(l.error_message, '') LIKE ?
		)`
		args = append(args, like, like, like, like)
	}
	if query.DateFrom.Valid {
		where += " AND COALESCE(l.started_at, l.created_at) >= ?"
		args = append(args, query.DateFrom.Time)
	}
	if query.DateTo.Valid {
		where += " AND COALESCE(l.started_at, l.created_at) < ?"
		args = append(args, query.DateTo.Time.AddDate(0, 0, 1))
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM mail_send_logs l `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	args = append(args, query.Limit, query.Offset)
	rows, err := s.db.Query(
		`SELECT l.id, l.user_id, l.account_id, a.email, l.message_id, m.subject, l.draft_id, l.source_message_id,
			l.compose_mode, l.smtp_message_id, l.recipients_json, l.subject, l.attachment_count, l.status,
			l.retry_status, l.retry_count, l.error_message, l.started_at, l.finished_at, l.created_at, l.updated_at
		FROM mail_send_logs l
		LEFT JOIN mail_accounts a ON a.user_id = l.user_id AND a.id = l.account_id
		LEFT JOIN mail_messages m ON m.user_id = l.user_id AND m.id = l.message_id `+where+`
		ORDER BY COALESCE(l.started_at, l.created_at) DESC, l.id DESC
		LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	logs := make([]MailSendLog, 0)
	for rows.Next() {
		item, err := scanMailSendLog(rows)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func scanMailSendLog(scanner interface {
	Scan(dest ...any) error
}) (MailSendLog, error) {
	var item MailSendLog
	err := scanner.Scan(
		&item.ID,
		&item.UserID,
		&item.AccountID,
		&item.AccountEmail,
		&item.MessageID,
		&item.MessageSubject,
		&item.DraftID,
		&item.SourceMessageID,
		&item.ComposeMode,
		&item.SMTPMessageID,
		&item.RecipientsJSON,
		&item.Subject,
		&item.AttachmentCount,
		&item.Status,
		&item.RetryStatus,
		&item.RetryCount,
		&item.ErrorMessage,
		&item.StartedAt,
		&item.FinishedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return MailSendLog{}, ErrNotFound
	}
	if err != nil {
		return MailSendLog{}, err
	}
	return item, nil
}

func normalizeSendLogComposeMode(value string) string {
	value = strings.TrimSpace(value)
	switch value {
	case "reply", "replyAll", "forward":
		return value
	default:
		return "new"
	}
}

func normalizeSendLogStatus(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "sending", "success", "failed", "local_save_failed":
		return value
	default:
		return "failed"
	}
}

func normalizeSendLogRetryStatus(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "none", "retryable", "retrying", "exhausted":
		return value
	default:
		return "none"
	}
}
