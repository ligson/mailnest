package storage

import (
	"database/sql"
	"strings"
)

func (s *Store) ListServerCleanupCandidates(userID, accountID int64, before sql.NullTime) ([]MailServerDeleteCandidate, error) {
	if !before.Valid {
		return []MailServerDeleteCandidate{}, nil
	}
	rows, err := s.db.Query(
		`SELECT id, user_id, account_id, folder, imap_uid, subject, from_addr, sent_at, received_at, raw_path
		FROM mail_messages
		WHERE user_id = ? AND account_id = ? AND folder = 'INBOX'
			AND imap_uid <> ''
			AND COALESCE(sent_at, received_at, created_at) < ?
		ORDER BY COALESCE(sent_at, received_at, created_at) ASC, id ASC`,
		userID,
		accountID,
		before.Time,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]MailServerDeleteCandidate, 0)
	for rows.Next() {
		var item MailServerDeleteCandidate
		if err := rows.Scan(
			&item.MessageID,
			&item.UserID,
			&item.AccountID,
			&item.Folder,
			&item.IMAPUID,
			&item.Subject,
			&item.FromAddr,
			&item.SentAt,
			&item.ReceivedAt,
			&item.RawPath,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateMailServerDeleteLog(params CreateMailServerDeleteLogParams) (MailServerDeleteLog, error) {
	logID, err := s.db.insertAndGetID(
		`INSERT INTO mail_server_delete_logs (
			user_id, account_id, sync_job_id, message_id, folder, imap_uid, subject, from_addr,
			sent_at, received_at, raw_path, raw_exists, status, reason, error_message, trigger_type
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		params.UserID,
		params.AccountID,
		nullInt64Value(params.SyncJobID),
		params.MessageID,
		strings.TrimSpace(params.Folder),
		strings.TrimSpace(params.IMAPUID),
		nullStringValue(params.Subject),
		nullStringValue(params.FromAddr),
		nullTimeValue(params.SentAt),
		nullTimeValue(params.ReceivedAt),
		nullStringValue(params.RawPath),
		boolToInt(params.RawExists),
		normalizeServerDeleteStatus(params.Status),
		strings.TrimSpace(params.Reason),
		nullIfEmpty(strings.TrimSpace(params.ErrorMessage)),
		normalizeServerDeleteTriggerType(params.TriggerType),
	)
	if err != nil {
		return MailServerDeleteLog{}, err
	}
	return s.FindMailServerDeleteLogByID(params.UserID, logID)
}

func (s *Store) UpdateMailServerDeleteLogStatus(params UpdateMailServerDeleteLogStatusParams) (MailServerDeleteLog, error) {
	result, err := s.db.Exec(
		`UPDATE mail_server_delete_logs
		SET status = ?, reason = ?, error_message = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		normalizeServerDeleteStatus(params.Status),
		strings.TrimSpace(params.Reason),
		nullIfEmpty(strings.TrimSpace(params.ErrorMessage)),
		params.UserID,
		params.ID,
	)
	if err != nil {
		return MailServerDeleteLog{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return MailServerDeleteLog{}, err
	}
	if count == 0 {
		return MailServerDeleteLog{}, ErrNotFound
	}
	return s.FindMailServerDeleteLogByID(params.UserID, params.ID)
}

func (s *Store) FindMailServerDeleteLogByID(userID, id int64) (MailServerDeleteLog, error) {
	row := s.db.QueryRow(
		`SELECT l.id, l.user_id, l.account_id, a.email, l.sync_job_id, l.message_id, l.folder, l.imap_uid,
			l.subject, l.from_addr, l.sent_at, l.received_at, l.raw_path, l.raw_exists, l.status, l.reason,
			l.error_message, l.trigger_type, l.created_at, l.updated_at
		FROM mail_server_delete_logs l
		LEFT JOIN mail_accounts a ON a.user_id = l.user_id AND a.id = l.account_id
		WHERE l.user_id = ? AND l.id = ?`,
		userID,
		id,
	)
	return scanMailServerDeleteLog(row)
}

func (s *Store) ListMailServerDeleteLogs(query ListMailServerDeleteLogsQuery) ([]MailServerDeleteLog, int, error) {
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
	if query.SyncJobID > 0 {
		where += " AND l.sync_job_id = ?"
		args = append(args, query.SyncJobID)
	}
	if query.Status = strings.TrimSpace(query.Status); query.Status != "" {
		where += " AND l.status = ?"
		args = append(args, query.Status)
	}
	if query.Keyword = strings.TrimSpace(query.Keyword); query.Keyword != "" {
		like := "%" + query.Keyword + "%"
		where += ` AND (
			COALESCE(l.subject, '') LIKE ?
			OR COALESCE(l.from_addr, '') LIKE ?
			OR COALESCE(l.imap_uid, '') LIKE ?
			OR COALESCE(l.reason, '') LIKE ?
			OR COALESCE(l.error_message, '') LIKE ?
		)`
		args = append(args, like, like, like, like, like)
	}
	if query.DateFrom.Valid {
		where += " AND l.created_at >= ?"
		args = append(args, query.DateFrom.Time)
	}
	if query.DateTo.Valid {
		where += " AND l.created_at < ?"
		args = append(args, query.DateTo.Time.AddDate(0, 0, 1))
	}
	countArgs := append([]any{}, args...)
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM mail_server_delete_logs l `+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 50
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	args = append(args, query.Limit, query.Offset)
	rows, err := s.db.Query(
		`SELECT l.id, l.user_id, l.account_id, a.email, l.sync_job_id, l.message_id, l.folder, l.imap_uid,
			l.subject, l.from_addr, l.sent_at, l.received_at, l.raw_path, l.raw_exists, l.status, l.reason,
			l.error_message, l.trigger_type, l.created_at, l.updated_at
		FROM mail_server_delete_logs l
		LEFT JOIN mail_accounts a ON a.user_id = l.user_id AND a.id = l.account_id
		`+where+`
		ORDER BY l.created_at DESC, l.id DESC
		LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]MailServerDeleteLog, 0)
	for rows.Next() {
		item, err := scanMailServerDeleteLog(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func scanMailServerDeleteLog(scanner interface {
	Scan(dest ...any) error
}) (MailServerDeleteLog, error) {
	var item MailServerDeleteLog
	var rawExists int
	err := scanner.Scan(
		&item.ID,
		&item.UserID,
		&item.AccountID,
		&item.AccountEmail,
		&item.SyncJobID,
		&item.MessageID,
		&item.Folder,
		&item.IMAPUID,
		&item.Subject,
		&item.FromAddr,
		&item.SentAt,
		&item.ReceivedAt,
		&item.RawPath,
		&rawExists,
		&item.Status,
		&item.Reason,
		&item.ErrorMessage,
		&item.TriggerType,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return MailServerDeleteLog{}, ErrNotFound
		}
		return MailServerDeleteLog{}, err
	}
	item.RawExists = rawExists == 1
	return item, nil
}
