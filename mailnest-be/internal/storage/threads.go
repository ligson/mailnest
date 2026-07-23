package storage

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

func (s *Store) CreateMailThread(params CreateMailThreadParams) (MailThread, error) {
	threadID, err := s.db.insertAndGetID(
		`INSERT INTO mail_threads (
			user_id, account_id, root_message_id, subject, normalized_subject, message_count, unread_count, has_attachments, last_message_at
		) VALUES (?, ?, ?, ?, ?, 0, 0, ?, ?)`,
		params.UserID,
		params.AccountID,
		nullInt64Value(params.RootMessageID),
		strings.TrimSpace(params.Subject),
		strings.TrimSpace(params.NormalizedSubject),
		boolToInt(params.HasAttachments),
		nullTimeValue(params.LastMessageAt),
	)
	if err != nil {
		return MailThread{}, err
	}
	return s.FindMailThreadByID(params.UserID, threadID)
}

func (s *Store) FindMailThreadByID(userID, id int64) (MailThread, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, account_id, root_message_id, subject, normalized_subject, message_count, unread_count, has_attachments, last_message_at, created_at, updated_at
		FROM mail_threads
		WHERE user_id = ? AND id = ?`,
		userID,
		id,
	)
	return scanMailThread(row)
}

func (s *Store) FindMailThreadBySourceMessageID(userID, messageID int64) (MailThread, error) {
	row := s.db.QueryRow(
		`SELECT t.id, t.user_id, t.account_id, t.root_message_id, t.subject, t.normalized_subject, t.message_count, t.unread_count, t.has_attachments, t.last_message_at, t.created_at, t.updated_at
		FROM mail_messages m
		INNER JOIN mail_threads t ON t.user_id = m.user_id AND t.id = m.thread_id
		WHERE m.user_id = ? AND m.id = ? AND m.thread_id IS NOT NULL`,
		userID,
		messageID,
	)
	return scanMailThread(row)
}

func (s *Store) FindMailThreadByReferencedMessageIDs(userID int64, messageIDs []string) (MailThread, error) {
	messageIDs = compactStrings(messageIDs)
	if len(messageIDs) == 0 {
		return MailThread{}, ErrNotFound
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(messageIDs)), ",")
	args := make([]any, 0, len(messageIDs)+1)
	args = append(args, userID)
	for _, id := range messageIDs {
		args = append(args, id)
	}
	row := s.db.QueryRow(
		`SELECT t.id, t.user_id, t.account_id, t.root_message_id, t.subject, t.normalized_subject, t.message_count, t.unread_count, t.has_attachments, t.last_message_at, t.created_at, t.updated_at
		FROM mail_messages m
		INNER JOIN mail_threads t ON t.user_id = m.user_id AND t.id = m.thread_id
		WHERE m.user_id = ? AND m.thread_id IS NOT NULL AND m.message_id IN (`+placeholders+`)
		ORDER BY m.id ASC
		LIMIT 1`,
		args...,
	)
	return scanMailThread(row)
}

func (s *Store) FindMailThreadByNormalizedSubject(userID, accountID int64, subject string) (MailThread, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return MailThread{}, ErrNotFound
	}
	row := s.db.QueryRow(
		`SELECT id, user_id, account_id, root_message_id, subject, normalized_subject, message_count, unread_count, has_attachments, last_message_at, created_at, updated_at
		FROM mail_threads
		WHERE user_id = ? AND account_id = ? AND normalized_subject = ?
		ORDER BY COALESCE(last_message_at, created_at) DESC, id DESC
		LIMIT 1`,
		userID,
		accountID,
		subject,
	)
	return scanMailThread(row)
}

func (s *Store) SetMailMessageThread(userID, messageID, threadID int64) error {
	result, err := s.db.Exec(
		`UPDATE mail_messages SET thread_id = ?, updated_at = CURRENT_TIMESTAMP WHERE user_id = ? AND id = ?`,
		threadID,
		userID,
		messageID,
	)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) RefreshMailThreadStats(userID, threadID int64) error {
	var messageCount int
	var unreadCount int
	var hasAttachments int
	var lastMessageValue any
	err := s.db.QueryRow(
		`SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN COALESCE(ms.is_read, 0) = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(MAX(CASE WHEN m.has_attachments = 1 THEN 1 ELSE 0 END), 0),
			MAX(COALESCE(m.sent_at, m.received_at, m.created_at))
		FROM mail_messages m
		LEFT JOIN mail_message_states ms ON ms.user_id = m.user_id AND ms.message_id = m.id
		WHERE m.user_id = ? AND m.thread_id = ?`,
		userID,
		threadID,
	).Scan(&messageCount, &unreadCount, &hasAttachments, &lastMessageValue)
	if err != nil {
		return err
	}
	if messageCount == 0 {
		_, err := s.db.Exec(`DELETE FROM mail_threads WHERE user_id = ? AND id = ?`, userID, threadID)
		return err
	}

	rootMessageID, subject := s.threadRootMessage(userID, threadID)
	_, err = s.db.Exec(
		`UPDATE mail_threads
		SET root_message_id = ?, subject = ?, message_count = ?, unread_count = ?, has_attachments = ?, last_message_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		nullInt64Value(rootMessageID),
		subject,
		messageCount,
		unreadCount,
		hasAttachments,
		nullTimeValue(dbValueToNullTime(lastMessageValue)),
		userID,
		threadID,
	)
	return err
}

func (s *Store) threadRootMessage(userID, threadID int64) (sql.NullInt64, string) {
	var id int64
	var subject sql.NullString
	err := s.db.QueryRow(
		`SELECT id, subject
		FROM mail_messages
		WHERE user_id = ? AND thread_id = ?
		ORDER BY COALESCE(sent_at, received_at, created_at) ASC, id ASC
		LIMIT 1`,
		userID,
		threadID,
	).Scan(&id, &subject)
	if err != nil {
		return sql.NullInt64{}, ""
	}
	return sql.NullInt64{Int64: id, Valid: true}, nullableStringValue(subject)
}

func (s *Store) ListMailThreads(query ListMailThreadsQuery) ([]MailThreadListItem, int, error) {
	where, args := buildThreadMessageWhere(query)
	where += " AND m.thread_id IS NOT NULL"

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(DISTINCT m.thread_id) FROM mail_messages m LEFT JOIN mail_message_states ms ON ms.user_id = m.user_id AND ms.message_id = m.id `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	listArgs := make([]any, 0, len(args)+3)
	listArgs = append(listArgs, query.UserID)
	listArgs = append(listArgs, args...)
	listArgs = append(listArgs, query.Limit, query.Offset)
	rows, err := s.db.Query(
		`SELECT t.id, t.user_id, t.account_id, t.root_message_id, t.subject, t.normalized_subject, t.message_count, t.unread_count, t.has_attachments, t.last_message_at, t.created_at, t.updated_at
		FROM mail_threads t
		WHERE t.user_id = ? AND t.id IN (
			SELECT DISTINCT m.thread_id
			FROM mail_messages m
			LEFT JOIN mail_message_states ms ON ms.user_id = m.user_id AND ms.message_id = m.id `+where+`
		)
		ORDER BY COALESCE(t.last_message_at, t.created_at) DESC, t.id DESC
		LIMIT ? OFFSET ?`,
		listArgs...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]MailThreadListItem, 0)
	for rows.Next() {
		thread, err := scanMailThread(rows)
		if err != nil {
			return nil, 0, err
		}
		latest, err := s.FindLatestMailMessageInThread(query.UserID, thread.ID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, 0, err
		}
		items = append(items, MailThreadListItem{
			Thread:        thread,
			LatestMessage: latest,
			Participants:  threadParticipants(latest),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Store) FindLatestMailMessageInThread(userID, threadID int64) (MailMessage, error) {
	row := s.db.QueryRow(
		`SELECT m.id, m.user_id, m.account_id, m.thread_id, m.local_folder_id, m.folder, m.imap_uid, m.message_id, m.subject, m.from_addr, m.to_addrs, m.cc_addrs,
			m.sent_at, m.received_at, m.has_attachments, m.text_body_path, m.html_body_path, m.raw_path, m.search_text,
			m.in_reply_to, m.references_header, m.source_message_id, m.compose_mode,
			COALESCE(ms.is_read, 0), COALESCE(ms.starred, 0), COALESCE(ms.is_spam, 0), ms.spam_at, ms.deleted_at, m.created_at, m.updated_at
		FROM mail_messages m
		LEFT JOIN mail_message_states ms ON ms.user_id = m.user_id AND ms.message_id = m.id
		WHERE m.user_id = ? AND m.thread_id = ?
		ORDER BY COALESCE(m.sent_at, m.received_at, m.created_at) DESC, m.id DESC
		LIMIT 1`,
		userID,
		threadID,
	)
	return scanMailMessage(row)
}

func (s *Store) ListMailThreadMessages(userID, threadID int64) ([]MailMessage, error) {
	if _, err := s.FindMailThreadByID(userID, threadID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT m.id, m.user_id, m.account_id, m.thread_id, m.local_folder_id, m.folder, m.imap_uid, m.message_id, m.subject, m.from_addr, m.to_addrs, m.cc_addrs,
			m.sent_at, m.received_at, m.has_attachments, m.text_body_path, m.html_body_path, m.raw_path, m.search_text,
			m.in_reply_to, m.references_header, m.source_message_id, m.compose_mode,
			COALESCE(ms.is_read, 0), COALESCE(ms.starred, 0), COALESCE(ms.is_spam, 0), ms.spam_at, ms.deleted_at, m.created_at, m.updated_at
		FROM mail_messages m
		LEFT JOIN mail_message_states ms ON ms.user_id = m.user_id AND ms.message_id = m.id
		WHERE m.user_id = ? AND m.thread_id = ?
		ORDER BY COALESCE(m.sent_at, m.received_at, m.created_at) ASC, m.id ASC`,
		userID,
		threadID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := make([]MailMessage, 0)
	for rows.Next() {
		message, err := scanMailMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (s *Store) ListMailMessagesForThreadRebuild(params RebuildThreadsParams) ([]MailMessage, error) {
	where := "WHERE m.user_id = ?"
	args := []any{params.UserID}
	if params.AccountID > 0 {
		where += " AND m.account_id = ?"
		args = append(args, params.AccountID)
	}
	if strings.TrimSpace(params.Scope) != "all" {
		where += " AND m.thread_id IS NULL"
	}
	rows, err := s.db.Query(
		`SELECT m.id, m.user_id, m.account_id, m.thread_id, m.local_folder_id, m.folder, m.imap_uid, m.message_id, m.subject, m.from_addr, m.to_addrs, m.cc_addrs,
			m.sent_at, m.received_at, m.has_attachments, m.text_body_path, m.html_body_path, m.raw_path, m.search_text,
			m.in_reply_to, m.references_header, m.source_message_id, m.compose_mode,
			COALESCE(ms.is_read, 0), COALESCE(ms.starred, 0), COALESCE(ms.is_spam, 0), ms.spam_at, ms.deleted_at, m.created_at, m.updated_at
		FROM mail_messages m
		LEFT JOIN mail_message_states ms ON ms.user_id = m.user_id AND ms.message_id = m.id `+where+`
		ORDER BY COALESCE(m.sent_at, m.received_at, m.created_at) ASC, m.id ASC`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := make([]MailMessage, 0)
	for rows.Next() {
		message, err := scanMailMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (s *Store) ResetMailThreads(userID, accountID int64) error {
	where := "WHERE user_id = ?"
	args := []any{userID}
	if accountID > 0 {
		where += " AND account_id = ?"
		args = append(args, accountID)
	}
	if _, err := s.db.Exec(`UPDATE mail_messages SET thread_id = NULL `+where, args...); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM mail_threads `+where, args...)
	return err
}

func (s *Store) CountMailThreads(userID int64) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM mail_threads WHERE user_id = ?`, userID).Scan(&count)
	return count, err
}

func buildThreadMessageWhere(query ListMailThreadsQuery) (string, []any) {
	where := "WHERE m.user_id = ?"
	args := []any{query.UserID}
	if query.AccountID > 0 {
		where += " AND m.account_id = ?"
		args = append(args, query.AccountID)
	}
	if query.FolderID > 0 {
		where += " AND m.local_folder_id = ?"
		args = append(args, query.FolderID)
	}
	switch strings.TrimSpace(query.SystemFolder) {
	case "inbox":
		where += " AND m.folder = ?"
		args = append(args, "INBOX")
	case "sent":
		where += ` AND m.folder IN (
			SELECT COALESCE(NULLIF(TRIM(sent_folder), ''), 'Sent')
			FROM mail_accounts
			WHERE user_id = ?
		)`
		args = append(args, query.UserID)
	case "attachments":
		where += " AND m.has_attachments = 1"
	case "trash":
		query.OnlyDeleted = true
	case "starred":
		query.Starred = sql.NullBool{Bool: true, Valid: true}
	case "spam":
		query.IsSpam = sql.NullBool{Bool: true, Valid: true}
	}
	if query.Keyword = strings.TrimSpace(query.Keyword); query.Keyword != "" {
		like := "%" + query.Keyword + "%"
		where += ` AND (
			COALESCE(m.subject, '') LIKE ?
			OR COALESCE(m.from_addr, '') LIKE ?
			OR COALESCE(m.to_addrs, '') LIKE ?
			OR COALESCE(m.cc_addrs, '') LIKE ?
			OR COALESCE(m.search_text, '') LIKE ?
		)`
		args = append(args, like, like, like, like, like)
	}
	if query.From = strings.TrimSpace(query.From); query.From != "" {
		where += " AND COALESCE(m.from_addr, '') LIKE ?"
		args = append(args, "%"+query.From+"%")
	}
	if query.Subject = strings.TrimSpace(query.Subject); query.Subject != "" {
		where += " AND COALESCE(m.subject, '') LIKE ?"
		args = append(args, "%"+query.Subject+"%")
	}
	if query.Body = strings.TrimSpace(query.Body); query.Body != "" {
		where += " AND COALESCE(m.search_text, '') LIKE ?"
		args = append(args, "%"+query.Body+"%")
	}
	if query.DateFrom.Valid {
		where += " AND COALESCE(m.sent_at, m.received_at, m.created_at) >= ?"
		args = append(args, query.DateFrom.Time)
	}
	if query.DateTo.Valid {
		where += " AND COALESCE(m.sent_at, m.received_at, m.created_at) < ?"
		args = append(args, query.DateTo.Time.AddDate(0, 0, 1))
	}
	if query.HasAttachments.Valid {
		where += " AND m.has_attachments = ?"
		args = append(args, boolToInt(query.HasAttachments.Bool))
	}
	if query.IsRead.Valid {
		where += " AND COALESCE(ms.is_read, 0) = ?"
		args = append(args, boolToInt(query.IsRead.Bool))
	}
	if query.Starred.Valid {
		where += " AND COALESCE(ms.starred, 0) = ?"
		args = append(args, boolToInt(query.Starred.Bool))
	}
	if query.IsSpam.Valid {
		where += " AND COALESCE(ms.is_spam, 0) = ?"
		args = append(args, boolToInt(query.IsSpam.Bool))
	} else {
		where += " AND COALESCE(ms.is_spam, 0) = 0"
	}
	if query.OnlyDeleted {
		where += " AND ms.deleted_at IS NOT NULL"
	} else if !query.IncludeDeleted {
		where += " AND ms.deleted_at IS NULL"
	}
	return where, args
}

func scanMailThread(scanner interface {
	Scan(dest ...any) error
}) (MailThread, error) {
	var thread MailThread
	var hasAttachments int
	err := scanner.Scan(
		&thread.ID,
		&thread.UserID,
		&thread.AccountID,
		&thread.RootMessageID,
		&thread.Subject,
		&thread.NormalizedSubject,
		&thread.MessageCount,
		&thread.UnreadCount,
		&hasAttachments,
		&thread.LastMessageAt,
		&thread.CreatedAt,
		&thread.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return MailThread{}, ErrNotFound
	}
	if err != nil {
		return MailThread{}, err
	}
	thread.HasAttachments = hasAttachments == 1
	return thread, nil
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
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

func threadParticipants(message MailMessage) []string {
	participants := []string{}
	if value := nullableStringValue(message.FromAddr); value != "" {
		participants = append(participants, value)
	}
	for _, value := range strings.Split(nullableStringValue(message.ToAddrs), ",") {
		value = strings.TrimSpace(value)
		if value != "" {
			participants = append(participants, value)
		}
	}
	return compactStrings(participants)
}

func nullableStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func dbValueToNullTime(value any) sql.NullTime {
	switch typed := value.(type) {
	case nil:
		return sql.NullTime{}
	case time.Time:
		return sql.NullTime{Time: typed, Valid: true}
	case string:
		return parseDBTimeString(typed)
	case []byte:
		return parseDBTimeString(string(typed))
	default:
		return sql.NullTime{}
	}
}

func parseDBTimeString(value string) sql.NullTime {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullTime{}
	}
	formats := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, format := range formats {
		if parsed, err := time.Parse(format, value); err == nil {
			return sql.NullTime{Time: parsed, Valid: true}
		}
	}
	return sql.NullTime{}
}
