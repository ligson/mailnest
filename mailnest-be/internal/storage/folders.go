package storage

import (
	"database/sql"
	"errors"
)

func (s *Store) CreateMailFolder(params CreateMailFolderParams) (MailFolder, error) {
	id, err := s.db.insertAndGetID(
		`INSERT INTO mail_folders (user_id, name, color, sort_order) VALUES (?, ?, ?, ?)`,
		params.UserID,
		params.Name,
		nullIfEmpty(params.Color),
		params.SortOrder,
	)
	if err != nil {
		return MailFolder{}, err
	}
	return s.FindMailFolderByID(params.UserID, id)
}

func (s *Store) ListMailFolders(userID int64) ([]MailFolder, error) {
	rows, err := s.db.Query(
		`SELECT f.id, f.user_id, f.name, f.color, f.sort_order,
			(SELECT COUNT(*) FROM mail_rules r WHERE r.user_id = f.user_id AND r.target_folder_id = f.id) AS rule_count,
			f.created_at, f.updated_at
		FROM mail_folders f
		WHERE f.user_id = ?
		ORDER BY f.sort_order ASC, f.name ASC, f.id ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	folders := make([]MailFolder, 0)
	for rows.Next() {
		folder, err := scanMailFolder(rows)
		if err != nil {
			return nil, err
		}
		folders = append(folders, folder)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return folders, nil
}

func (s *Store) FindMailFolderByID(userID, id int64) (MailFolder, error) {
	row := s.db.QueryRow(
		`SELECT f.id, f.user_id, f.name, f.color, f.sort_order,
			(SELECT COUNT(*) FROM mail_rules r WHERE r.user_id = f.user_id AND r.target_folder_id = f.id) AS rule_count,
			f.created_at, f.updated_at
		FROM mail_folders f
		WHERE f.user_id = ? AND f.id = ?`,
		userID,
		id,
	)
	return scanMailFolder(row)
}

func (s *Store) UpdateMailFolder(userID, id int64, params CreateMailFolderParams) (MailFolder, error) {
	result, err := s.db.Exec(
		`UPDATE mail_folders
		SET name = ?, color = ?, sort_order = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		params.Name,
		nullIfEmpty(params.Color),
		params.SortOrder,
		userID,
		id,
	)
	if err != nil {
		return MailFolder{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return MailFolder{}, err
	}
	if count == 0 {
		return MailFolder{}, ErrNotFound
	}
	return s.FindMailFolderByID(userID, id)
}

func (s *Store) DeleteMailFolder(userID, id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var ruleCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM mail_rules WHERE user_id = ? AND target_folder_id = ?`, userID, id).Scan(&ruleCount); err != nil {
		return err
	}
	if ruleCount > 0 {
		return ErrMailFolderHasRules
	}

	if _, err := tx.Exec(`UPDATE mail_messages SET local_folder_id = NULL, updated_at = CURRENT_TIMESTAMP WHERE user_id = ? AND local_folder_id = ?`, userID, id); err != nil {
		return err
	}
	result, err := tx.Exec(`DELETE FROM mail_folders WHERE user_id = ? AND id = ?`, userID, id)
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
	return tx.Commit()
}

func scanMailFolder(scanner interface {
	Scan(dest ...any) error
}) (MailFolder, error) {
	var folder MailFolder
	var ruleCount int
	err := scanner.Scan(
		&folder.ID,
		&folder.UserID,
		&folder.Name,
		&folder.Color,
		&folder.SortOrder,
		&ruleCount,
		&folder.CreatedAt,
		&folder.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return MailFolder{}, ErrNotFound
	}
	if err != nil {
		return MailFolder{}, err
	}
	folder.RuleCount = ruleCount
	return folder, nil
}
