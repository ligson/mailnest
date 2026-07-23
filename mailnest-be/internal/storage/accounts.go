package storage

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// CreateMailAccount 保存邮箱账号配置；调用方必须先完成凭据加密。
func (s *Store) CreateMailAccount(account MailAccount) (MailAccount, error) {
	if strings.TrimSpace(account.Provider) == "" {
		account.Provider = "custom"
	}
	if strings.TrimSpace(account.AuthType) == "" {
		account.AuthType = "password"
	}
	id, err := s.db.insertAndGetID(
		`INSERT INTO mail_accounts (
			user_id, provider, auth_type, display_name, email, imap_host, imap_port, imap_tls, imap_username,
			imap_password_encrypted, smtp_host, smtp_port, smtp_tls, smtp_starttls, smtp_username,
			smtp_password_encrypted, sent_folder, signature_html, oauth_access_token_encrypted, oauth_refresh_token_encrypted, oauth_expires_at,
			poll_interval_minutes, enabled, cleanup_enabled, cleanup_retention_days
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		account.UserID,
		account.Provider,
		account.AuthType,
		account.DisplayName,
		account.Email,
		account.IMAPHost,
		account.IMAPPort,
		boolToInt(account.IMAPTLS),
		account.IMAPUsername,
		account.IMAPPasswordEncoded,
		account.SMTPHost,
		normalizeSMTPPort(account.SMTPPort),
		boolToInt(account.SMTPTLS),
		boolToInt(account.SMTPStartTLS),
		account.SMTPUsername,
		account.SMTPPasswordEncoded,
		normalizeSentFolder(account.SentFolder),
		account.SignatureHTML,
		nullStringValue(account.OAuthAccessToken),
		nullStringValue(account.OAuthRefreshToken),
		nullTimeValue(account.OAuthExpiresAt),
		account.PollIntervalMinutes,
		boolToInt(account.Enabled),
		boolToInt(account.CleanupEnabled),
		normalizeRetentionDays(account.CleanupRetentionDays),
	)
	if err != nil {
		return MailAccount{}, err
	}

	return s.FindMailAccountByID(account.UserID, id)
}

func (s *Store) ListMailAccounts(userID int64) ([]MailAccount, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, provider, auth_type, display_name, email, imap_host, imap_port, imap_tls, imap_username,
			imap_password_encrypted, smtp_host, smtp_port, smtp_tls, smtp_starttls, smtp_username,
			smtp_password_encrypted, sent_folder, signature_html, oauth_access_token_encrypted, oauth_refresh_token_encrypted, oauth_expires_at,
			poll_interval_minutes, enabled, last_sync_at, last_sync_status,
			last_sync_error, full_sync_status, full_sync_total, full_sync_processed, full_sync_new_count,
			full_sync_started_at, full_sync_finished_at, full_sync_error, cleanup_enabled, cleanup_retention_days,
			created_at, updated_at
		FROM mail_accounts
		WHERE user_id = ?
		ORDER BY created_at DESC, id DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := make([]MailAccount, 0)
	for rows.Next() {
		account, err := scanMailAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return accounts, nil
}

func (s *Store) ListDueMailAccounts(limit int) ([]MailAccount, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	// 只挑选启用且没有全量同步运行中的账号，避免定时收取和全量同步互相抢同一账号。
	rows, err := s.db.Query(
		`SELECT id, user_id, provider, auth_type, display_name, email, imap_host, imap_port, imap_tls, imap_username,
			imap_password_encrypted, smtp_host, smtp_port, smtp_tls, smtp_starttls, smtp_username,
			smtp_password_encrypted, sent_folder, signature_html, oauth_access_token_encrypted, oauth_refresh_token_encrypted, oauth_expires_at,
			poll_interval_minutes, enabled, last_sync_at, last_sync_status,
			last_sync_error, full_sync_status, full_sync_total, full_sync_processed, full_sync_new_count,
			full_sync_started_at, full_sync_finished_at, full_sync_error, cleanup_enabled, cleanup_retention_days,
			created_at, updated_at
		FROM mail_accounts
		`+s.db.dueMailAccountsWhere()+`
		ORDER BY last_sync_at IS NOT NULL ASC, last_sync_at ASC, id ASC
		LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := make([]MailAccount, 0)
	for rows.Next() {
		account, err := scanMailAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (s *Store) FindMailAccountByID(userID, id int64) (MailAccount, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, provider, auth_type, display_name, email, imap_host, imap_port, imap_tls, imap_username,
			imap_password_encrypted, smtp_host, smtp_port, smtp_tls, smtp_starttls, smtp_username,
			smtp_password_encrypted, sent_folder, signature_html, oauth_access_token_encrypted, oauth_refresh_token_encrypted, oauth_expires_at,
			poll_interval_minutes, enabled, last_sync_at, last_sync_status,
			last_sync_error, full_sync_status, full_sync_total, full_sync_processed, full_sync_new_count,
			full_sync_started_at, full_sync_finished_at, full_sync_error, cleanup_enabled, cleanup_retention_days,
			created_at, updated_at
		FROM mail_accounts
		WHERE user_id = ? AND id = ?`,
		userID,
		id,
	)
	return scanMailAccount(row)
}

func (s *Store) UpdateMailAccount(account MailAccount) (MailAccount, error) {
	result, err := s.db.Exec(
		`UPDATE mail_accounts
		SET display_name = ?, email = ?, imap_host = ?, imap_port = ?, imap_tls = ?, imap_username = ?,
			imap_password_encrypted = ?, smtp_host = ?, smtp_port = ?, smtp_tls = ?, smtp_starttls = ?, smtp_username = ?,
			smtp_password_encrypted = ?, sent_folder = ?, signature_html = ?, poll_interval_minutes = ?, enabled = ?, cleanup_enabled = ?,
			cleanup_retention_days = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		account.DisplayName,
		account.Email,
		account.IMAPHost,
		account.IMAPPort,
		boolToInt(account.IMAPTLS),
		account.IMAPUsername,
		account.IMAPPasswordEncoded,
		account.SMTPHost,
		normalizeSMTPPort(account.SMTPPort),
		boolToInt(account.SMTPTLS),
		boolToInt(account.SMTPStartTLS),
		account.SMTPUsername,
		account.SMTPPasswordEncoded,
		normalizeSentFolder(account.SentFolder),
		account.SignatureHTML,
		account.PollIntervalMinutes,
		boolToInt(account.Enabled),
		boolToInt(account.CleanupEnabled),
		normalizeRetentionDays(account.CleanupRetentionDays),
		account.UserID,
		account.ID,
	)
	if err != nil {
		return MailAccount{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return MailAccount{}, err
	}
	if count == 0 {
		return MailAccount{}, ErrNotFound
	}
	return s.FindMailAccountByID(account.UserID, account.ID)
}

func (s *Store) UpdateMailAccountOAuthTokens(userID, id int64, accessToken, refreshToken string, expiresAt time.Time) error {
	_, err := s.db.Exec(
		`UPDATE mail_accounts
		SET oauth_access_token_encrypted = ?, oauth_refresh_token_encrypted = ?, oauth_expires_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		accessToken,
		refreshToken,
		expiresAt,
		userID,
		id,
	)
	return err
}

func (s *Store) DeleteMailAccount(userID, id int64) error {
	result, err := s.db.Exec(`DELETE FROM mail_accounts WHERE user_id = ? AND id = ?`, userID, id)
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

func (s *Store) UpdateMailAccountSyncStatus(userID, id int64, status, errMessage string) error {
	_, err := s.db.Exec(
		`UPDATE mail_accounts
		SET last_sync_at = CURRENT_TIMESTAMP, last_sync_status = ?, last_sync_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		status,
		nullIfEmpty(errMessage),
		userID,
		id,
	)
	return err
}

func (s *Store) StartMailAccountFullSync(userID, id, total int64) error {
	_, err := s.db.Exec(
		`UPDATE mail_accounts
		SET full_sync_status = 'running', full_sync_total = ?, full_sync_processed = 0, full_sync_new_count = 0,
			full_sync_started_at = CURRENT_TIMESTAMP, full_sync_finished_at = NULL, full_sync_error = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		total,
		userID,
		id,
	)
	return err
}

func (s *Store) UpdateMailAccountFullSyncProgress(userID, id int64, total, processed, newCount int) error {
	_, err := s.db.Exec(
		`UPDATE mail_accounts
		SET full_sync_total = ?, full_sync_processed = ?, full_sync_new_count = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		total,
		processed,
		newCount,
		userID,
		id,
	)
	return err
}

func (s *Store) FinishMailAccountFullSync(userID, id int64, status, errMessage string) error {
	_, err := s.db.Exec(
		`UPDATE mail_accounts
		SET full_sync_status = ?, full_sync_finished_at = CURRENT_TIMESTAMP, full_sync_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		status,
		nullIfEmpty(errMessage),
		userID,
		id,
	)
	return err
}

func (s *Store) MarkStaleFullSyncsFailed() error {
	_, err := s.db.Exec(
		`UPDATE mail_accounts
		SET full_sync_status = 'failed',
			full_sync_finished_at = CURRENT_TIMESTAMP,
			full_sync_error = '服务重启后已重置未完成的全量同步，请重新启动',
			updated_at = CURRENT_TIMESTAMP
		WHERE full_sync_status = 'running'`,
	)
	return err
}

func (s *Store) ListSyncedInboxUIDsBefore(userID, accountID int64, before time.Time) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT imap_uid
		FROM mail_messages
		WHERE user_id = ? AND account_id = ? AND folder = 'INBOX'
			AND COALESCE(sent_at, received_at, created_at) < ?
		ORDER BY COALESCE(sent_at, received_at, created_at) ASC, id ASC`,
		userID,
		accountID,
		before,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	uids := make([]string, 0)
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		uids = append(uids, uid)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return uids, nil
}

func scanMailAccount(scanner interface {
	Scan(dest ...any) error
}) (MailAccount, error) {
	var account MailAccount
	var imapTLS int
	var smtpTLS int
	var smtpStartTLS int
	var enabled int
	var cleanupEnabled int
	err := scanner.Scan(
		&account.ID,
		&account.UserID,
		&account.Provider,
		&account.AuthType,
		&account.DisplayName,
		&account.Email,
		&account.IMAPHost,
		&account.IMAPPort,
		&imapTLS,
		&account.IMAPUsername,
		&account.IMAPPasswordEncoded,
		&account.SMTPHost,
		&account.SMTPPort,
		&smtpTLS,
		&smtpStartTLS,
		&account.SMTPUsername,
		&account.SMTPPasswordEncoded,
		&account.SentFolder,
		&account.SignatureHTML,
		&account.OAuthAccessToken,
		&account.OAuthRefreshToken,
		&account.OAuthExpiresAt,
		&account.PollIntervalMinutes,
		&enabled,
		&account.LastSyncAt,
		&account.LastSyncStatus,
		&account.LastSyncError,
		&account.FullSyncStatus,
		&account.FullSyncTotal,
		&account.FullSyncProcessed,
		&account.FullSyncNewCount,
		&account.FullSyncStartedAt,
		&account.FullSyncFinishedAt,
		&account.FullSyncError,
		&cleanupEnabled,
		&account.CleanupRetentionDays,
		&account.CreatedAt,
		&account.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return MailAccount{}, ErrNotFound
	}
	if err != nil {
		return MailAccount{}, err
	}
	account.IMAPTLS = imapTLS == 1
	account.SMTPTLS = smtpTLS == 1
	account.SMTPStartTLS = smtpStartTLS == 1
	account.SMTPPort = normalizeSMTPPort(account.SMTPPort)
	account.Enabled = enabled == 1
	account.SentFolder = normalizeSentFolder(account.SentFolder)
	account.CleanupEnabled = cleanupEnabled == 1
	account.CleanupRetentionDays = normalizeRetentionDays(account.CleanupRetentionDays)
	if strings.TrimSpace(account.FullSyncStatus) == "" {
		account.FullSyncStatus = "idle"
	}
	return account, nil
}
