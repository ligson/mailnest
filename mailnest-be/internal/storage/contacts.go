package storage

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

func (s *Store) CreateContact(params CreateContactParams) (Contact, error) {
	email := strings.TrimSpace(params.Email)
	emailKey := normalizeEmailKey(email)
	if strings.TrimSpace(params.Source) == "" {
		params.Source = "manual"
	}
	seenAt := params.SeenAt
	if !seenAt.Valid {
		now := time.Now()
		seenAt = sql.NullTime{Time: now, Valid: true}
	}
	id, err := s.db.insertAndGetID(
		`INSERT INTO contacts (
			user_id, email, email_key, display_name, nickname, phone, company, notes, source, first_seen_at, last_seen_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		params.UserID,
		email,
		emailKey,
		nullIfEmpty(params.DisplayName),
		nullIfEmpty(params.Nickname),
		nullIfEmpty(params.Phone),
		nullIfEmpty(params.Company),
		nullIfEmpty(params.Notes),
		params.Source,
		nullTimeValue(seenAt),
		nullTimeValue(seenAt),
	)
	if err != nil {
		return Contact{}, err
	}
	return s.FindContactByID(params.UserID, id)
}

func (s *Store) UpsertContactSeen(params CreateContactParams) (Contact, error) {
	email := strings.TrimSpace(params.Email)
	emailKey := normalizeEmailKey(email)
	if emailKey == "" {
		return Contact{}, ErrNotFound
	}
	if strings.TrimSpace(params.Source) == "" {
		params.Source = "auto"
	}
	seenAt := params.SeenAt
	if !seenAt.Valid {
		seenAt = sql.NullTime{Time: time.Now(), Valid: true}
	}
	_, err := s.db.Exec(
		s.db.upsertContactSeenSQL(),
		params.UserID,
		email,
		emailKey,
		nullIfEmpty(params.DisplayName),
		params.Source,
		nullTimeValue(seenAt),
		nullTimeValue(seenAt),
	)
	if err != nil {
		return Contact{}, err
	}
	return s.FindContactByEmail(params.UserID, email)
}

func (s *Store) ListContacts(query ListContactsQuery) ([]Contact, int, error) {
	where := "WHERE user_id = ?"
	args := []any{query.UserID}
	if query.Keyword = strings.TrimSpace(query.Keyword); query.Keyword != "" {
		like := "%" + query.Keyword + "%"
		where += ` AND (
			COALESCE(email, '') LIKE ?
			OR COALESCE(display_name, '') LIKE ?
			OR COALESCE(nickname, '') LIKE ?
			OR COALESCE(phone, '') LIKE ?
			OR COALESCE(company, '') LIKE ?
			OR COALESCE(notes, '') LIKE ?
		)`
		args = append(args, like, like, like, like, like, like)
	}
	countArgs := append([]any{}, args...)
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM contacts `+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if query.Limit <= 0 || query.Limit > 1000 {
		query.Limit = 100
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	args = append(args, query.Limit, query.Offset)
	rows, err := s.db.Query(
		`SELECT id, user_id, email, email_key, display_name, nickname, phone, company, notes, source,
			first_seen_at, last_seen_at, created_at, updated_at
		FROM contacts `+where+`
		ORDER BY LOWER(COALESCE(nickname, display_name, email)) ASC, id ASC
		LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	contacts := make([]Contact, 0)
	for rows.Next() {
		contact, err := scanContact(rows)
		if err != nil {
			return nil, 0, err
		}
		contacts = append(contacts, contact)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return contacts, total, nil
}

func (s *Store) FindContactByID(userID, id int64) (Contact, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, email, email_key, display_name, nickname, phone, company, notes, source,
			first_seen_at, last_seen_at, created_at, updated_at
		FROM contacts
		WHERE user_id = ? AND id = ?`,
		userID,
		id,
	)
	return scanContact(row)
}

func (s *Store) FindContactByEmail(userID int64, email string) (Contact, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, email, email_key, display_name, nickname, phone, company, notes, source,
			first_seen_at, last_seen_at, created_at, updated_at
		FROM contacts
		WHERE user_id = ? AND email_key = ?`,
		userID,
		normalizeEmailKey(email),
	)
	return scanContact(row)
}

func (s *Store) UpdateContact(contact Contact) (Contact, error) {
	email := strings.TrimSpace(contact.Email)
	emailKey := normalizeEmailKey(email)
	result, err := s.db.Exec(
		`UPDATE contacts
		SET email = ?, email_key = ?, display_name = ?, nickname = ?, phone = ?, company = ?, notes = ?,
			source = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		email,
		emailKey,
		nullStringValue(contact.DisplayName),
		nullStringValue(contact.Nickname),
		nullStringValue(contact.Phone),
		nullStringValue(contact.Company),
		nullStringValue(contact.Notes),
		contact.Source,
		contact.UserID,
		contact.ID,
	)
	if err != nil {
		return Contact{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Contact{}, err
	}
	if count == 0 {
		return Contact{}, ErrNotFound
	}
	return s.FindContactByID(contact.UserID, contact.ID)
}

func (s *Store) DeleteContact(userID, id int64) error {
	result, err := s.db.Exec(`DELETE FROM contacts WHERE user_id = ? AND id = ?`, userID, id)
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

func scanContact(scanner interface {
	Scan(dest ...any) error
}) (Contact, error) {
	var contact Contact
	err := scanner.Scan(
		&contact.ID,
		&contact.UserID,
		&contact.Email,
		&contact.EmailKey,
		&contact.DisplayName,
		&contact.Nickname,
		&contact.Phone,
		&contact.Company,
		&contact.Notes,
		&contact.Source,
		&contact.FirstSeenAt,
		&contact.LastSeenAt,
		&contact.CreatedAt,
		&contact.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Contact{}, ErrNotFound
	}
	if err != nil {
		return Contact{}, err
	}
	if strings.TrimSpace(contact.Source) == "" {
		contact.Source = "manual"
	}
	return contact, nil
}
