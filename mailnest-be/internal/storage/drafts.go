package storage

import (
	"database/sql"
	"errors"
	"fmt"
)

func (s *Store) SaveMailDraft(params SaveMailDraftParams) (MailDraft, error) {
	if params.UserID <= 0 {
		return MailDraft{}, fmt.Errorf("用户 ID 不能为空")
	}
	if params.AccountID <= 0 {
		return MailDraft{}, fmt.Errorf("请选择发件邮箱账号")
	}
	if _, err := s.FindMailAccountByID(params.UserID, params.AccountID); err != nil {
		return MailDraft{}, err
	}
	if params.SourceMessageID.Valid {
		if _, err := s.FindMailMessageByID(params.UserID, params.SourceMessageID.Int64); err != nil {
			return MailDraft{}, err
		}
	}
	if params.ComposeMode == "" {
		params.ComposeMode = "new"
	}
	if params.ToAddrsJSON == "" {
		params.ToAddrsJSON = "[]"
	}
	if params.CCAddrsJSON == "" {
		params.CCAddrsJSON = "[]"
	}
	if params.BCCAddrsJSON == "" {
		params.BCCAddrsJSON = "[]"
	}
	if params.ForwardAttachmentIDsJSON == "" {
		params.ForwardAttachmentIDsJSON = "[]"
	}
	if params.LocalAttachmentNamesJSON == "" {
		params.LocalAttachmentNamesJSON = "[]"
	}

	if params.ID > 0 {
		result, err := s.db.Exec(
			`UPDATE mail_drafts
			SET account_id = ?, compose_mode = ?, source_message_id = ?, to_addrs_json = ?, cc_addrs_json = ?,
				bcc_addrs_json = ?, subject = ?, text_body = ?, html_body = ?, forward_attachment_ids_json = ?,
				local_attachment_names_json = ?, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND id = ?`,
			params.AccountID,
			params.ComposeMode,
			nullInt64Value(params.SourceMessageID),
			params.ToAddrsJSON,
			params.CCAddrsJSON,
			params.BCCAddrsJSON,
			params.Subject,
			params.TextBody,
			params.HTMLBody,
			params.ForwardAttachmentIDsJSON,
			params.LocalAttachmentNamesJSON,
			params.UserID,
			params.ID,
		)
		if err != nil {
			return MailDraft{}, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return MailDraft{}, err
		}
		if count == 0 {
			return MailDraft{}, ErrNotFound
		}
		return s.FindMailDraftByID(params.UserID, params.ID)
	}

	id, err := s.db.insertAndGetID(
		`INSERT INTO mail_drafts (
			user_id, account_id, compose_mode, source_message_id, to_addrs_json, cc_addrs_json, bcc_addrs_json,
			subject, text_body, html_body, forward_attachment_ids_json, local_attachment_names_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		params.UserID,
		params.AccountID,
		params.ComposeMode,
		nullInt64Value(params.SourceMessageID),
		params.ToAddrsJSON,
		params.CCAddrsJSON,
		params.BCCAddrsJSON,
		params.Subject,
		params.TextBody,
		params.HTMLBody,
		params.ForwardAttachmentIDsJSON,
		params.LocalAttachmentNamesJSON,
	)
	if err != nil {
		return MailDraft{}, err
	}
	return s.FindMailDraftByID(params.UserID, id)
}

func (s *Store) ListMailDrafts(query ListMailDraftsQuery) ([]MailDraft, int, error) {
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM mail_drafts WHERE user_id = ?`, query.UserID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(
		`SELECT id, user_id, account_id, compose_mode, source_message_id, to_addrs_json, cc_addrs_json, bcc_addrs_json,
			subject, text_body, html_body, forward_attachment_ids_json, local_attachment_names_json, created_at, updated_at
		FROM mail_drafts
		WHERE user_id = ?
		ORDER BY updated_at DESC, id DESC
		LIMIT ? OFFSET ?`,
		query.UserID,
		query.Limit,
		query.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	drafts := make([]MailDraft, 0)
	for rows.Next() {
		draft, err := scanMailDraft(rows)
		if err != nil {
			return nil, 0, err
		}
		drafts = append(drafts, draft)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return drafts, total, nil
}

func (s *Store) FindMailDraftByID(userID, id int64) (MailDraft, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, account_id, compose_mode, source_message_id, to_addrs_json, cc_addrs_json, bcc_addrs_json,
			subject, text_body, html_body, forward_attachment_ids_json, local_attachment_names_json, created_at, updated_at
		FROM mail_drafts
		WHERE user_id = ? AND id = ?`,
		userID,
		id,
	)
	return scanMailDraft(row)
}

func (s *Store) DeleteMailDraft(userID, id int64) error {
	result, err := s.db.Exec(`DELETE FROM mail_drafts WHERE user_id = ? AND id = ?`, userID, id)
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

func scanMailDraft(scanner interface {
	Scan(dest ...any) error
}) (MailDraft, error) {
	var draft MailDraft
	err := scanner.Scan(
		&draft.ID,
		&draft.UserID,
		&draft.AccountID,
		&draft.ComposeMode,
		&draft.SourceMessageID,
		&draft.ToAddrsJSON,
		&draft.CCAddrsJSON,
		&draft.BCCAddrsJSON,
		&draft.Subject,
		&draft.TextBody,
		&draft.HTMLBody,
		&draft.ForwardAttachmentIDsJSON,
		&draft.LocalAttachmentNamesJSON,
		&draft.CreatedAt,
		&draft.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return MailDraft{}, ErrNotFound
	}
	if err != nil {
		return MailDraft{}, err
	}
	return draft, nil
}
