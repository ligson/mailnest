package storage

import (
	"database/sql"
	"errors"
	"strings"
)

func (s *Store) CreateUser(username, email, passwordHash string) (User, error) {
	isAdmin, err := s.nextUserShouldBeAdmin()
	if err != nil {
		return User{}, err
	}
	id, err := s.db.insertAndGetID(
		`INSERT INTO users (username, email, password_hash, is_admin, enabled) VALUES (?, ?, ?, ?, ?)`,
		username,
		email,
		passwordHash,
		boolToInt(isAdmin),
		boolToInt(true),
	)
	if err != nil {
		return User{}, err
	}

	return s.FindUserByID(id)
}

func (s *Store) nextUserShouldBeAdmin() (bool, error) {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

func (s *Store) FindUserByAccount(account string) (User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, email, password_hash, nickname, avatar_path, bio, ui_theme, is_admin, enabled, created_at, updated_at FROM users WHERE username = ? OR email = ?`,
		account,
		account,
	)
	return scanUser(row)
}

func (s *Store) FindUserByID(id int64) (User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, email, password_hash, nickname, avatar_path, bio, ui_theme, is_admin, enabled, created_at, updated_at FROM users WHERE id = ?`,
		id,
	)
	return scanUser(row)
}

func (s *Store) ensureFirstUserAdmin() error {
	var adminCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE is_admin = ?`, boolToInt(true)).Scan(&adminCount); err != nil {
		return err
	}
	if adminCount > 0 {
		return nil
	}

	var firstID sql.NullInt64
	if err := s.db.QueryRow(`SELECT MIN(id) FROM users`).Scan(&firstID); err != nil {
		return err
	}
	if !firstID.Valid {
		return nil
	}
	_, err := s.db.Exec(`UPDATE users SET is_admin = ?, enabled = ? WHERE id = ?`, boolToInt(true), boolToInt(true), firstID.Int64)
	return err
}

func (s *Store) UpdateUserProfile(id int64, nickname, bio, uiTheme string) (User, error) {
	result, err := s.db.Exec(
		`UPDATE users SET nickname = ?, bio = ?, ui_theme = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		nullIfEmpty(nickname),
		nullIfEmpty(bio),
		normalizeUITheme(uiTheme),
		id,
	)
	if err != nil {
		return User{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return User{}, err
	}
	if count == 0 {
		return User{}, ErrNotFound
	}
	return s.FindUserByID(id)
}

func (s *Store) UpdateUserAvatarPath(id int64, avatarPath string) (User, error) {
	result, err := s.db.Exec(
		`UPDATE users SET avatar_path = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		nullIfEmpty(avatarPath),
		id,
	)
	if err != nil {
		return User{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return User{}, err
	}
	if count == 0 {
		return User{}, ErrNotFound
	}
	return s.FindUserByID(id)
}

func (s *Store) UpdateUserPasswordHash(id int64, passwordHash string) error {
	result, err := s.db.Exec(
		`UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		passwordHash,
		id,
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

func (s *Store) UpdateUserEnabled(id int64, enabled bool) (User, error) {
	result, err := s.db.Exec(
		`UPDATE users SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		boolToInt(enabled),
		id,
	)
	if err != nil {
		return User{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return User{}, err
	}
	if count == 0 {
		return User{}, ErrNotFound
	}
	return s.FindUserByID(id)
}

func (s *Store) ListAdminUserSummaries() ([]AdminUserSummary, error) {
	rows, err := s.db.Query(
		`SELECT
			u.id, u.username, u.email, u.password_hash, u.nickname, u.avatar_path, u.bio, u.ui_theme, u.is_admin, u.enabled, u.created_at, u.updated_at,
			COALESCE(ma.account_count, 0),
			COALESCE(mm.message_count, 0),
			COALESCE(att.attachment_count, 0),
			COALESCE(att.attachment_bytes, 0),
			COALESCE(ct.contact_count, 0),
			COALESCE(mf.folder_count, 0),
			COALESCE(mr.rule_count, 0),
			mm.last_message_at,
			ma.last_sync_at
		FROM users u
		LEFT JOIN (
			SELECT user_id, COUNT(*) AS account_count, MAX(last_sync_at) AS last_sync_at
			FROM mail_accounts
			GROUP BY user_id
		) ma ON ma.user_id = u.id
		LEFT JOIN (
			SELECT user_id, COUNT(*) AS message_count, MAX(COALESCE(sent_at, received_at, created_at)) AS last_message_at
			FROM mail_messages
			GROUP BY user_id
		) mm ON mm.user_id = u.id
		LEFT JOIN (
			SELECT user_id, COUNT(*) AS attachment_count, COALESCE(SUM(size), 0) AS attachment_bytes
			FROM mail_attachments
			GROUP BY user_id
		) att ON att.user_id = u.id
		LEFT JOIN (
			SELECT user_id, COUNT(*) AS contact_count
			FROM contacts
			GROUP BY user_id
		) ct ON ct.user_id = u.id
		LEFT JOIN (
			SELECT user_id, COUNT(*) AS folder_count
			FROM mail_folders
			GROUP BY user_id
		) mf ON mf.user_id = u.id
		LEFT JOIN (
			SELECT user_id, COUNT(*) AS rule_count
			FROM mail_rules
			GROUP BY user_id
		) mr ON mr.user_id = u.id
		ORDER BY u.created_at ASC, u.id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := []AdminUserSummary{}
	for rows.Next() {
		var item AdminUserSummary
		if err := rows.Scan(
			&item.User.ID,
			&item.User.Username,
			&item.User.Email,
			&item.User.PasswordHash,
			&item.User.Nickname,
			&item.User.AvatarPath,
			&item.User.Bio,
			&item.User.UITheme,
			&item.User.IsAdmin,
			&item.User.Enabled,
			&item.User.CreatedAt,
			&item.User.UpdatedAt,
			&item.MailAccountCount,
			&item.MessageCount,
			&item.AttachmentCount,
			&item.AttachmentBytes,
			&item.ContactCount,
			&item.FolderCount,
			&item.RuleCount,
			&item.LastMessageAt,
			&item.LastSyncAt,
		); err != nil {
			return nil, err
		}
		item.User.UITheme = normalizeUITheme(item.User.UITheme)
		summaries = append(summaries, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return summaries, nil
}

func scanUser(scanner interface {
	Scan(dest ...any) error
}) (User, error) {
	var user User
	err := scanner.Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.Nickname,
		&user.AvatarPath,
		&user.Bio,
		&user.UITheme,
		&user.IsAdmin,
		&user.Enabled,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	user.UITheme = normalizeUITheme(user.UITheme)
	return user, nil
}

func normalizeUITheme(value string) string {
	theme := strings.TrimSpace(value)
	switch theme {
	case "sky", "grape", "ember", "graphite", "qinghua", "cinnabar", "ink", "daishan":
		return theme
	default:
		return "forest"
	}
}
