package storage

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

func (s *Store) ListAttachments(query ListAttachmentsQuery) ([]AttachmentListItem, int, error) {
	where := "WHERE a.user_id = ?"
	args := []any{query.UserID}
	if query.AccountID > 0 {
		where += " AND m.account_id = ?"
		args = append(args, query.AccountID)
	}
	if query.FolderID > 0 {
		where += " AND m.local_folder_id = ?"
		args = append(args, query.FolderID)
	}
	if query.Keyword = strings.TrimSpace(query.Keyword); query.Keyword != "" {
		where += " AND COALESCE(a.filename, '') LIKE ?"
		args = append(args, "%"+query.Keyword+"%")
	}
	if query.ContentType = strings.TrimSpace(query.ContentType); query.ContentType != "" {
		where += " AND COALESCE(a.content_type, '') LIKE ?"
		args = append(args, "%"+query.ContentType+"%")
	}
	if query.Inline.Valid {
		where += " AND a.inline = ?"
		args = append(args, boolToInt(query.Inline.Bool))
	}
	if query.DateFrom.Valid {
		where += " AND COALESCE(m.sent_at, m.received_at, m.created_at) >= ?"
		args = append(args, query.DateFrom.Time)
	}
	if query.DateTo.Valid {
		where += " AND COALESCE(m.sent_at, m.received_at, m.created_at) < ?"
		args = append(args, query.DateTo.Time.AddDate(0, 0, 1))
	}
	countArgs := append([]any{}, args...)
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM mail_attachments a JOIN mail_messages m ON m.id = a.message_id `+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 20
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	args = append(args, query.Limit, query.Offset)
	rows, err := s.db.Query(
		`SELECT a.id, a.user_id, a.message_id, a.filename, a.content_type, a.content_id, a.inline, a.size, a.file_path, a.created_at,
			m.account_id, m.local_folder_id, m.subject, m.from_addr, m.sent_at, m.received_at, m.created_at
		FROM mail_attachments a
		JOIN mail_messages m ON m.id = a.message_id
		`+where+`
		ORDER BY COALESCE(m.sent_at, m.received_at, m.created_at) DESC, a.id DESC
		LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]AttachmentListItem, 0)
	for rows.Next() {
		var item AttachmentListItem
		var inline int
		var sentAt sql.NullTime
		var receivedAt sql.NullTime
		var createdAt time.Time
		if err := rows.Scan(
			&item.Attachment.ID,
			&item.Attachment.UserID,
			&item.Attachment.MessageID,
			&item.Attachment.Filename,
			&item.Attachment.ContentType,
			&item.Attachment.ContentID,
			&inline,
			&item.Attachment.Size,
			&item.Attachment.FilePath,
			&item.Attachment.CreatedAt,
			&item.AccountID,
			&item.LocalFolderID,
			&item.MessageSubject,
			&item.MessageFrom,
			&sentAt,
			&receivedAt,
			&createdAt,
		); err != nil {
			return nil, 0, err
		}
		item.Attachment.Inline = inline == 1
		switch {
		case sentAt.Valid:
			item.MessageTime = sentAt
		case receivedAt.Valid:
			item.MessageTime = receivedAt
		default:
			item.MessageTime = sql.NullTime{Time: createdAt, Valid: true}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Store) CreateMailAttachment(params CreateMailAttachmentParams) (MailAttachment, error) {
	if strings.TrimSpace(params.Filename) == "" {
		params.Filename = "attachment"
	}
	id, err := s.db.insertAndGetID(
		`INSERT INTO mail_attachments (
			user_id, message_id, filename, content_type, content_id, inline, size, file_path
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		params.UserID,
		params.MessageID,
		params.Filename,
		nullIfEmpty(params.ContentType),
		nullIfEmpty(params.ContentID),
		boolToInt(params.Inline),
		params.Size,
		params.FilePath,
	)
	if err != nil {
		return MailAttachment{}, err
	}
	return s.FindMailAttachmentByID(params.UserID, params.MessageID, id)
}

func (s *Store) ListMailAttachments(userID, messageID int64) ([]MailAttachment, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, message_id, filename, content_type, content_id, inline, size, file_path, created_at
		FROM mail_attachments
		WHERE user_id = ? AND message_id = ?
		ORDER BY inline DESC, id ASC`,
		userID,
		messageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	attachments := make([]MailAttachment, 0)
	for rows.Next() {
		attachment, err := scanMailAttachment(rows)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return attachments, nil
}

func (s *Store) FindMailAttachmentByID(userID, messageID, id int64) (MailAttachment, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, message_id, filename, content_type, content_id, inline, size, file_path, created_at
		FROM mail_attachments
		WHERE user_id = ? AND message_id = ? AND id = ?`,
		userID,
		messageID,
		id,
	)
	return scanMailAttachment(row)
}

func scanMailAttachment(scanner interface {
	Scan(dest ...any) error
}) (MailAttachment, error) {
	var attachment MailAttachment
	var inline int
	err := scanner.Scan(
		&attachment.ID,
		&attachment.UserID,
		&attachment.MessageID,
		&attachment.Filename,
		&attachment.ContentType,
		&attachment.ContentID,
		&inline,
		&attachment.Size,
		&attachment.FilePath,
		&attachment.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return MailAttachment{}, ErrNotFound
	}
	if err != nil {
		return MailAttachment{}, err
	}
	attachment.Inline = inline == 1
	return attachment, nil
}
