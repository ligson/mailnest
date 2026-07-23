package storage

import (
	"database/sql"
	"errors"
	"strings"
)

func (s *Store) CreateUser(username, email, passwordHash string) (User, error) {
	id, err := s.db.insertAndGetID(
		`INSERT INTO users (username, email, password_hash) VALUES (?, ?, ?)`,
		username,
		email,
		passwordHash,
	)
	if err != nil {
		return User{}, err
	}

	return s.FindUserByID(id)
}

func (s *Store) FindUserByAccount(account string) (User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, email, password_hash, nickname, avatar_path, bio, ui_theme, created_at, updated_at FROM users WHERE username = ? OR email = ?`,
		account,
		account,
	)
	return scanUser(row)
}

func (s *Store) FindUserByID(id int64) (User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, email, password_hash, nickname, avatar_path, bio, ui_theme, created_at, updated_at FROM users WHERE id = ?`,
		id,
	)
	return scanUser(row)
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
