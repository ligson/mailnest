package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	if query.GroupID > 0 {
		// 不可仅按成员表 ID 筛选，必须同时验证分组和联系人的用户归属。
		var count int64
		if err := s.db.gormDB.Model(&ContactGroup{}).Where("user_id=? AND id=?", query.UserID, query.GroupID).Count(&count).Error; err != nil {
			return nil, 0, err
		}
		if count == 0 {
			return nil, 0, ErrNotFound
		}
		where += " AND EXISTS (SELECT 1 FROM contact_group_members gm WHERE gm.user_id=contacts.user_id AND gm.contact_id=contacts.id AND gm.group_id=?)"
		args = append(args, query.GroupID)
	}
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
	err := s.db.gormDB.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
		var contact contactModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id=? AND id=?", userID, id).First(&contact).Error; err != nil {
			return groupNotFound(err)
		}
		if err := tx.Where("user_id=? AND contact_id=?", userID, id).Delete(&contactGroupMember{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id=? AND id=?", userID, id).Delete(&contactModel{}).Error
	})
	// GORM 写入不经过旧 SQL 包装层，成功提交后显式失效该用户的名称解析缓存。
	if err == nil {
		s.db.cacheVersions.bump(cacheMutation{domain: "contacts", userID: userID})
	}
	return err
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
