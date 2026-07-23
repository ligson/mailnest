package storage

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// InsertMailMessageIfNew 使用账号、目录和 IMAP UID 去重，避免重复同步同一封邮件。
func (s *Store) InsertMailMessageIfNew(params CreateMailMessageParams) (MailMessage, bool, error) {
	result, err := s.db.Exec(
		s.db.insertIgnoreSQL(
			"mail_messages",
			[]string{
				"user_id", "account_id", "folder", "imap_uid", "message_id", "subject", "from_addr", "to_addrs", "cc_addrs",
				"sent_at", "received_at", "has_attachments", "text_body_path", "html_body_path", "raw_path", "search_text",
				"in_reply_to", "references_header", "source_message_id", "compose_mode",
			},
			[]string{"account_id", "folder", "imap_uid"},
		),
		params.UserID,
		params.AccountID,
		params.Folder,
		params.IMAPUID,
		nullIfEmpty(params.MessageID),
		nullIfEmpty(params.Subject),
		nullIfEmpty(params.FromAddr),
		nullIfEmpty(params.ToAddrs),
		nullIfEmpty(params.CCAddrs),
		nullTimeValue(params.SentAt),
		nullTimeValue(params.ReceivedAt),
		boolToInt(params.HasAttachments),
		nullIfEmpty(params.TextBodyPath),
		nullIfEmpty(params.HTMLBodyPath),
		nullIfEmpty(params.RawPath),
		nullIfEmpty(params.SearchText),
		nullIfEmpty(params.InReplyTo),
		nullIfEmpty(params.References),
		nullInt64Value(params.SourceMessageID),
		nullIfEmpty(params.ComposeMode),
	)
	if err != nil {
		return MailMessage{}, false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return MailMessage{}, false, err
	}

	message, err := s.FindMailMessageByUID(params.UserID, params.AccountID, params.Folder, params.IMAPUID)
	if err != nil {
		return MailMessage{}, false, err
	}

	return message, rows > 0, nil
}

func (s *Store) FindMailMessageByUID(userID, accountID int64, folder, uid string) (MailMessage, error) {
	row := s.db.QueryRow(
		`SELECT m.id, m.user_id, m.account_id, m.local_folder_id, m.folder, m.imap_uid, m.message_id, m.subject, m.from_addr, m.to_addrs, m.cc_addrs,
			m.sent_at, m.received_at, m.has_attachments, m.text_body_path, m.html_body_path, m.raw_path, m.search_text,
			m.in_reply_to, m.references_header, m.source_message_id, m.compose_mode,
			COALESCE(ms.is_read, 0), COALESCE(ms.starred, 0), COALESCE(ms.is_spam, 0), ms.spam_at, ms.deleted_at, m.created_at, m.updated_at
		FROM mail_messages m
		LEFT JOIN mail_message_states ms ON ms.user_id = m.user_id AND ms.message_id = m.id
		WHERE m.user_id = ? AND m.account_id = ? AND m.folder = ? AND m.imap_uid = ?`,
		userID,
		accountID,
		folder,
		uid,
	)
	return scanMailMessage(row)
}

func (s *Store) ListMailMessages(userID int64, accountID int64, limit, offset int) ([]MailMessage, int, error) {
	return s.ListMailMessagesByQuery(ListMailMessagesQuery{
		UserID:    userID,
		AccountID: accountID,
		Limit:     limit,
		Offset:    offset,
	})
}

func (s *Store) ListMailMessagesByQuery(query ListMailMessagesQuery) ([]MailMessage, int, error) {
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
		// 主搜索框覆盖主题、发件人、收件人、抄送和预构建 search_text；
		// 前端切换搜索范围时，再使用下面的精确字段过滤。
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

	countArgs := append([]any{}, args...)
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM mail_messages m LEFT JOIN mail_message_states ms ON ms.user_id = m.user_id AND ms.message_id = m.id `+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	args = append(args, query.Limit, query.Offset)

	selectSQL := `SELECT m.id, m.user_id, m.account_id, m.local_folder_id, m.folder, m.imap_uid, m.message_id, m.subject, m.from_addr, m.to_addrs, m.cc_addrs,
			m.sent_at, m.received_at, m.has_attachments, m.text_body_path, m.html_body_path, m.raw_path, m.search_text,
			m.in_reply_to, m.references_header, m.source_message_id, m.compose_mode,
			COALESCE(ms.is_read, 0), COALESCE(ms.starred, 0), COALESCE(ms.is_spam, 0), ms.spam_at, ms.deleted_at, m.created_at, m.updated_at
		FROM mail_messages m
		LEFT JOIN mail_message_states ms ON ms.user_id = m.user_id AND ms.message_id = m.id ` + where + `
		ORDER BY COALESCE(m.sent_at, m.received_at, m.created_at) DESC, m.id DESC
		LIMIT ? OFFSET ?`
	if query.SummaryOnly {
		// 邮件列表默认只读摘要字段，详情页再读取正文文件，避免首屏被大正文拖慢。
		selectSQL = `SELECT m.id, m.user_id, m.account_id, m.local_folder_id, m.folder, m.imap_uid, m.subject, m.from_addr, m.to_addrs,
			m.sent_at, m.received_at, m.has_attachments, COALESCE(ms.is_read, 0), COALESCE(ms.starred, 0), COALESCE(ms.is_spam, 0), ms.spam_at, ms.deleted_at, m.created_at, m.updated_at
		FROM mail_messages m
		LEFT JOIN mail_message_states ms ON ms.user_id = m.user_id AND ms.message_id = m.id ` + where + `
		ORDER BY COALESCE(m.sent_at, m.received_at, m.created_at) DESC, m.id DESC
		LIMIT ? OFFSET ?`
	}

	rows, err := s.db.Query(selectSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	messages := make([]MailMessage, 0)
	for rows.Next() {
		scan := scanMailMessage
		if query.SummaryOnly {
			scan = scanMailMessageSummary
		}
		message, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

func (s *Store) FindMailMessageByID(userID, id int64) (MailMessage, error) {
	row := s.db.QueryRow(
		`SELECT m.id, m.user_id, m.account_id, m.local_folder_id, m.folder, m.imap_uid, m.message_id, m.subject, m.from_addr, m.to_addrs, m.cc_addrs,
			m.sent_at, m.received_at, m.has_attachments, m.text_body_path, m.html_body_path, m.raw_path, m.search_text,
			m.in_reply_to, m.references_header, m.source_message_id, m.compose_mode,
			COALESCE(ms.is_read, 0), COALESCE(ms.starred, 0), COALESCE(ms.is_spam, 0), ms.spam_at, ms.deleted_at, m.created_at, m.updated_at
		FROM mail_messages m
		LEFT JOIN mail_message_states ms ON ms.user_id = m.user_id AND ms.message_id = m.id
		WHERE m.user_id = ? AND m.id = ?`,
		userID,
		id,
	)
	return scanMailMessage(row)
}

func (s *Store) ListMailMessagesWithRawContent(limit int) ([]MailMessage, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	rows, err := s.db.Query(
		`SELECT m.id, m.user_id, m.account_id, m.local_folder_id, m.folder, m.imap_uid, m.message_id, m.subject, m.from_addr, m.to_addrs, m.cc_addrs,
			m.sent_at, m.received_at, m.has_attachments, m.text_body_path, m.html_body_path, m.raw_path, m.search_text,
			m.in_reply_to, m.references_header, m.source_message_id, m.compose_mode,
			COALESCE(ms.is_read, 0), COALESCE(ms.starred, 0), COALESCE(ms.is_spam, 0), ms.spam_at, ms.deleted_at, m.created_at, m.updated_at
		FROM mail_messages m
		LEFT JOIN mail_message_states ms ON ms.user_id = m.user_id AND ms.message_id = m.id
		WHERE m.raw_path IS NOT NULL
		ORDER BY m.id ASC
		LIMIT ?`,
		limit,
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

func (s *Store) UpdateMailMessageParsedContent(params UpdateMailMessageContentParams) error {
	result, err := s.db.Exec(
		`UPDATE mail_messages
		SET message_id = ?, subject = ?, from_addr = ?, to_addrs = ?, cc_addrs = ?,
			text_body_path = ?, html_body_path = ?, search_text = ?, in_reply_to = ?, references_header = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		nullIfEmpty(params.MessageID),
		nullIfEmpty(params.Subject),
		nullIfEmpty(params.FromAddr),
		nullIfEmpty(params.ToAddrs),
		nullIfEmpty(params.CCAddrs),
		nullIfEmpty(params.TextBodyPath),
		nullIfEmpty(params.HTMLBodyPath),
		nullIfEmpty(params.SearchText),
		nullIfEmpty(params.InReplyTo),
		nullIfEmpty(params.References),
		params.UserID,
		params.ID,
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

func (s *Store) UpdateMailMessageHasAttachments(userID, messageID int64, hasAttachments bool) error {
	_, err := s.db.Exec(
		`UPDATE mail_messages
		SET has_attachments = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		boolToInt(hasAttachments),
		userID,
		messageID,
	)
	return err
}

func (s *Store) UpsertMailMessageState(userID, messageID int64, isRead, starred, isSpam *bool, deletedAt *time.Time) error {
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM mail_messages WHERE user_id = ? AND id = ?`, userID, messageID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return ErrNotFound
	}
	if _, err := s.db.Exec(
		s.db.insertIgnoreSQL(
			"mail_message_states",
			[]string{"user_id", "message_id"},
			[]string{"user_id", "message_id"},
		),
		userID,
		messageID,
	); err != nil {
		return err
	}
	if isRead != nil {
		if _, err := s.db.Exec(
			`UPDATE mail_message_states
			SET is_read = ?, read_at = CASE WHEN ? = 1 THEN CURRENT_TIMESTAMP ELSE NULL END, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND message_id = ?`,
			boolToInt(*isRead),
			boolToInt(*isRead),
			userID,
			messageID,
		); err != nil {
			return err
		}
	}
	if starred != nil {
		if _, err := s.db.Exec(
			`UPDATE mail_message_states SET starred = ?, updated_at = CURRENT_TIMESTAMP WHERE user_id = ? AND message_id = ?`,
			boolToInt(*starred),
			userID,
			messageID,
		); err != nil {
			return err
		}
	}
	if isSpam != nil {
		if _, err := s.db.Exec(
			`UPDATE mail_message_states
			SET is_spam = ?, spam_at = CASE WHEN ? = 1 THEN CURRENT_TIMESTAMP ELSE NULL END, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND message_id = ?`,
			boolToInt(*isSpam),
			boolToInt(*isSpam),
			userID,
			messageID,
		); err != nil {
			return err
		}
	}
	if deletedAt != nil {
		if _, err := s.db.Exec(
			`UPDATE mail_message_states SET deleted_at = ?, updated_at = CURRENT_TIMESTAMP WHERE user_id = ? AND message_id = ?`,
			*deletedAt,
			userID,
			messageID,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) SetMailMessageFolderState(userID, messageID int64, folderID sql.NullInt64) error {
	return s.UpdateMailMessageFolder(userID, messageID, folderID)
}

func (s *Store) MarkMailMessageDeleted(userID, messageID int64, deleted bool) error {
	if deleted {
		now := time.Now()
		return s.UpsertMailMessageState(userID, messageID, nil, nil, nil, &now)
	}
	return s.ClearMailMessageDeleted(userID, messageID)
}

func (s *Store) ClearMailMessageDeleted(userID, messageID int64) error {
	if err := s.UpsertMailMessageState(userID, messageID, nil, nil, nil, nil); err != nil {
		return err
	}
	_, err := s.db.Exec(
		`UPDATE mail_message_states SET deleted_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE user_id = ? AND message_id = ?`,
		userID,
		messageID,
	)
	return err
}

func (s *Store) BatchUpdateMailMessageStates(userID int64, messageIDs []int64, action string, folderID sql.NullInt64) (MessageBatchActionResult, error) {
	if len(messageIDs) == 0 {
		return MessageBatchActionResult{}, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(messageIDs)), ",")
	args := make([]any, 0, len(messageIDs)+2)
	for _, id := range messageIDs {
		args = append(args, id)
	}
	args = append(args, userID)
	rows, err := s.db.Query(`SELECT id FROM mail_messages WHERE id IN (`+placeholders+`) AND user_id = ?`, args...)
	if err != nil {
		return MessageBatchActionResult{}, err
	}
	defer rows.Close()
	ownedIDs := make([]int64, 0, len(messageIDs))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return MessageBatchActionResult{}, err
		}
		ownedIDs = append(ownedIDs, id)
	}
	if err := rows.Err(); err != nil {
		return MessageBatchActionResult{}, err
	}
	matched := len(ownedIDs)
	changed := 0
	skipped := len(messageIDs) - matched
	switch action {
	case "mark_read":
		value := true
		for _, id := range ownedIDs {
			if err := s.UpsertMailMessageState(userID, id, &value, nil, nil, nil); err != nil {
				return MessageBatchActionResult{}, err
			}
			changed++
		}
	case "mark_unread":
		value := false
		for _, id := range ownedIDs {
			if err := s.UpsertMailMessageState(userID, id, &value, nil, nil, nil); err != nil {
				return MessageBatchActionResult{}, err
			}
			changed++
		}
	case "star":
		value := true
		for _, id := range ownedIDs {
			if err := s.UpsertMailMessageState(userID, id, nil, &value, nil, nil); err != nil {
				return MessageBatchActionResult{}, err
			}
			changed++
		}
	case "unstar":
		value := false
		for _, id := range ownedIDs {
			if err := s.UpsertMailMessageState(userID, id, nil, &value, nil, nil); err != nil {
				return MessageBatchActionResult{}, err
			}
			changed++
		}
	case "mark_spam":
		value := true
		for _, id := range ownedIDs {
			if err := s.UpsertMailMessageState(userID, id, nil, nil, &value, nil); err != nil {
				return MessageBatchActionResult{}, err
			}
			changed++
		}
	case "unmark_spam":
		value := false
		for _, id := range ownedIDs {
			if err := s.UpsertMailMessageState(userID, id, nil, nil, &value, nil); err != nil {
				return MessageBatchActionResult{}, err
			}
			changed++
		}
	case "move_folder":
		if !folderID.Valid {
			return MessageBatchActionResult{}, errors.New("folder id required")
		}
		for _, id := range ownedIDs {
			if err := s.UpdateMailMessageFolder(userID, id, folderID); err != nil {
				return MessageBatchActionResult{}, err
			}
			changed++
		}
	case "delete":
		for _, id := range ownedIDs {
			now := time.Now()
			if err := s.UpsertMailMessageState(userID, id, nil, nil, nil, &now); err != nil {
				return MessageBatchActionResult{}, err
			}
			changed++
		}
	case "restore":
		for _, id := range ownedIDs {
			if err := s.ClearMailMessageDeleted(userID, id); err != nil {
				return MessageBatchActionResult{}, err
			}
			changed++
		}
	default:
		return MessageBatchActionResult{}, errors.New("unsupported batch action")
	}
	return MessageBatchActionResult{MatchedCount: matched, ChangedCount: changed, SkippedCount: skipped}, nil
}

func (s *Store) PreviewMailMessageBatch(userID int64, messageIDs []int64) (MessageBatchPreview, error) {
	preview := MessageBatchPreview{}
	if len(messageIDs) == 0 {
		return preview, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(messageIDs)), ",")
	args := make([]any, 0, len(messageIDs)+1)
	for _, id := range messageIDs {
		args = append(args, id)
	}
	args = append(args, userID)
	rows, err := s.db.Query(
		`SELECT m.local_folder_id, COALESCE(ms.is_read, 0), COALESCE(ms.starred, 0), COALESCE(ms.is_spam, 0), ms.deleted_at
		FROM mail_messages m
		LEFT JOIN mail_message_states ms ON ms.user_id = m.user_id AND ms.message_id = m.id
		WHERE m.id IN (`+placeholders+`) AND m.user_id = ?`,
		args...,
	)
	if err != nil {
		return preview, err
	}
	defer rows.Close()
	type folderAgg struct{ count int }
	folders := make(map[int64]*folderAgg)
	for rows.Next() {
		var folderID sql.NullInt64
		var isRead int
		var starred int
		var isSpam int
		var deletedAt sql.NullTime
		if err := rows.Scan(&folderID, &isRead, &starred, &isSpam, &deletedAt); err != nil {
			return preview, err
		}
		preview.Total++
		if isRead == 1 {
			preview.ReadCount++
		} else {
			preview.UnreadCount++
		}
		if starred == 1 {
			preview.StarredCount++
		}
		if isSpam == 1 {
			preview.SpamCount++
		}
		if deletedAt.Valid {
			preview.DeletedCount++
		}
		if folderID.Valid {
			if _, ok := folders[folderID.Int64]; !ok {
				folders[folderID.Int64] = &folderAgg{}
			}
			folders[folderID.Int64].count++
		}
	}
	if err := rows.Err(); err != nil {
		return preview, err
	}
	for id, agg := range folders {
		name := ""
		if err := s.db.QueryRow(`SELECT name FROM mail_folders WHERE id = ? AND user_id = ?`, id, userID).Scan(&name); err == nil {
			preview.FolderCounts = append(preview.FolderCounts, MessageBatchFolderCount{FolderID: id, Name: name, Count: agg.count})
		}
	}
	return preview, nil
}

func (s *Store) UpdateMailMessageFolder(userID, messageID int64, folderID sql.NullInt64) error {
	result, err := s.db.Exec(
		`UPDATE mail_messages
		SET local_folder_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		nullInt64Value(folderID),
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

func scanMailMessage(scanner interface {
	Scan(dest ...any) error
}) (MailMessage, error) {
	var message MailMessage
	var hasAttachments int
	var isRead int
	var starred int
	var isSpam int
	err := scanner.Scan(
		&message.ID,
		&message.UserID,
		&message.AccountID,
		&message.LocalFolderID,
		&message.Folder,
		&message.IMAPUID,
		&message.MessageID,
		&message.Subject,
		&message.FromAddr,
		&message.ToAddrs,
		&message.CCAddrs,
		&message.SentAt,
		&message.ReceivedAt,
		&hasAttachments,
		&message.TextBodyPath,
		&message.HTMLBodyPath,
		&message.RawPath,
		&message.SearchText,
		&message.InReplyTo,
		&message.References,
		&message.SourceMessageID,
		&message.ComposeMode,
		&isRead,
		&starred,
		&isSpam,
		&message.SpamAt,
		&message.DeletedAt,
		&message.CreatedAt,
		&message.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return MailMessage{}, ErrNotFound
	}
	if err != nil {
		return MailMessage{}, err
	}
	message.HasAttachments = hasAttachments == 1
	message.IsRead = isRead == 1
	message.Starred = starred == 1
	message.IsSpam = isSpam == 1
	return message, nil
}

func scanMailMessageSummary(scanner interface {
	Scan(dest ...any) error
}) (MailMessage, error) {
	var message MailMessage
	var hasAttachments int
	var isRead int
	var starred int
	var isSpam int
	err := scanner.Scan(
		&message.ID,
		&message.UserID,
		&message.AccountID,
		&message.LocalFolderID,
		&message.Folder,
		&message.IMAPUID,
		&message.Subject,
		&message.FromAddr,
		&message.ToAddrs,
		&message.SentAt,
		&message.ReceivedAt,
		&hasAttachments,
		&isRead,
		&starred,
		&isSpam,
		&message.SpamAt,
		&message.DeletedAt,
		&message.CreatedAt,
		&message.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return MailMessage{}, ErrNotFound
	}
	if err != nil {
		return MailMessage{}, err
	}
	message.HasAttachments = hasAttachments == 1
	message.IsRead = isRead == 1
	message.Starred = starred == 1
	message.IsSpam = isSpam == 1
	return message, nil
}
