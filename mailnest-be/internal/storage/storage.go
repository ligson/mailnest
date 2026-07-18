package storage

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	ID           int64
	Username     string
	Email        string
	PasswordHash string
	Nickname     sql.NullString
	AvatarPath   sql.NullString
	Bio          sql.NullString
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type MailAccount struct {
	ID                   int64
	UserID               int64
	Provider             string
	AuthType             string
	DisplayName          string
	Email                string
	IMAPHost             string
	IMAPPort             int
	IMAPTLS              bool
	IMAPUsername         string
	IMAPPasswordEncoded  string
	SMTPHost             string
	SMTPPort             int
	SMTPTLS              bool
	SMTPStartTLS         bool
	SMTPUsername         string
	SMTPPasswordEncoded  string
	SentFolder           string
	SignatureHTML        string
	OAuthAccessToken     sql.NullString
	OAuthRefreshToken    sql.NullString
	OAuthExpiresAt       sql.NullTime
	PollIntervalMinutes  int
	Enabled              bool
	LastSyncAt           sql.NullTime
	LastSyncStatus       sql.NullString
	LastSyncError        sql.NullString
	FullSyncStatus       string
	FullSyncTotal        int
	FullSyncProcessed    int
	FullSyncNewCount     int
	FullSyncStartedAt    sql.NullTime
	FullSyncFinishedAt   sql.NullTime
	FullSyncError        sql.NullString
	CleanupEnabled       bool
	CleanupRetentionDays int
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type MailMessage struct {
	ID             int64
	UserID         int64
	AccountID      int64
	LocalFolderID  sql.NullInt64
	Folder         string
	IMAPUID        string
	MessageID      sql.NullString
	Subject        sql.NullString
	FromAddr       sql.NullString
	ToAddrs        sql.NullString
	CCAddrs        sql.NullString
	SentAt         sql.NullTime
	ReceivedAt     sql.NullTime
	HasAttachments bool
	TextBodyPath   sql.NullString
	HTMLBodyPath   sql.NullString
	RawPath        sql.NullString
	SearchText     sql.NullString
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type MailAttachment struct {
	ID          int64
	UserID      int64
	MessageID   int64
	Filename    string
	ContentType sql.NullString
	ContentID   sql.NullString
	Inline      bool
	Size        int64
	FilePath    string
	CreatedAt   time.Time
}

type MailFolder struct {
	ID        int64
	UserID    int64
	Name      string
	Color     sql.NullString
	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Contact struct {
	ID          int64
	UserID      int64
	Email       string
	EmailKey    string
	DisplayName sql.NullString
	Nickname    sql.NullString
	Phone       sql.NullString
	Company     sql.NullString
	Notes       sql.NullString
	Source      string
	FirstSeenAt sql.NullTime
	LastSeenAt  sql.NullTime
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type MailRule struct {
	ID             int64
	UserID         int64
	Name           string
	Enabled        bool
	MatchMode      string
	TargetFolderID int64
	SortOrder      int
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Conditions     []MailRuleCondition
}

type MailRuleCondition struct {
	ID       int64
	RuleID   int64
	Field    string
	Operator string
	Value    string
}

type CreateMailMessageParams struct {
	UserID         int64
	AccountID      int64
	Folder         string
	IMAPUID        string
	MessageID      string
	Subject        string
	FromAddr       string
	ToAddrs        string
	CCAddrs        string
	SentAt         sql.NullTime
	ReceivedAt     sql.NullTime
	HasAttachments bool
	TextBodyPath   string
	HTMLBodyPath   string
	RawPath        string
	SearchText     string
}

type UpdateMailMessageContentParams struct {
	UserID       int64
	ID           int64
	MessageID    string
	Subject      string
	FromAddr     string
	ToAddrs      string
	CCAddrs      string
	TextBodyPath string
	HTMLBodyPath string
	SearchText   string
}

type ListMailMessagesQuery struct {
	UserID         int64
	AccountID      int64
	FolderID       int64
	SystemFolder   string
	Keyword        string
	From           string
	Subject        string
	Body           string
	DateFrom       sql.NullTime
	DateTo         sql.NullTime
	HasAttachments sql.NullBool
	Limit          int
	Offset         int
	SummaryOnly    bool
}

type CreateMailAttachmentParams struct {
	UserID      int64
	MessageID   int64
	Filename    string
	ContentType string
	ContentID   string
	Inline      bool
	Size        int64
	FilePath    string
}

type CreateMailFolderParams struct {
	UserID    int64
	Name      string
	Color     string
	SortOrder int
}

type CreateContactParams struct {
	UserID      int64
	Email       string
	DisplayName string
	Nickname    string
	Phone       string
	Company     string
	Notes       string
	Source      string
	SeenAt      sql.NullTime
}

type ListContactsQuery struct {
	UserID  int64
	Keyword string
	Limit   int
	Offset  int
}

type CreateMailRuleParams struct {
	UserID         int64
	Name           string
	Enabled        bool
	MatchMode      string
	TargetFolderID int64
	SortOrder      int
	Conditions     []CreateMailRuleConditionParams
}

type CreateMailRuleConditionParams struct {
	Field    string
	Operator string
	Value    string
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", sqliteDSN(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	if _, err := db.Exec(`PRAGMA journal_mode = WAL; PRAGMA synchronous = NORMAL; PRAGMA busy_timeout = 10000; PRAGMA temp_store = MEMORY;`); err != nil {
		_ = db.Close()
		return nil, err
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func sqliteDSN(path string) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + "_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_temp_store=MEMORY"
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	nickname TEXT,
	avatar_path TEXT,
	bio TEXT,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS mail_accounts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	provider TEXT NOT NULL DEFAULT 'custom',
	auth_type TEXT NOT NULL DEFAULT 'password',
	display_name TEXT NOT NULL,
	email TEXT NOT NULL,
	imap_host TEXT NOT NULL,
	imap_port INTEGER NOT NULL,
	imap_tls INTEGER NOT NULL DEFAULT 1,
	imap_username TEXT NOT NULL,
	imap_password_encrypted TEXT NOT NULL,
	smtp_host TEXT NOT NULL DEFAULT '',
	smtp_port INTEGER NOT NULL DEFAULT 587,
	smtp_tls INTEGER NOT NULL DEFAULT 0,
	smtp_starttls INTEGER NOT NULL DEFAULT 1,
	smtp_username TEXT NOT NULL DEFAULT '',
	smtp_password_encrypted TEXT NOT NULL DEFAULT '',
	sent_folder TEXT NOT NULL DEFAULT 'Sent',
	signature_html TEXT NOT NULL DEFAULT '',
	oauth_access_token_encrypted TEXT,
	oauth_refresh_token_encrypted TEXT,
	oauth_expires_at DATETIME,
	poll_interval_minutes INTEGER NOT NULL DEFAULT 10,
	enabled INTEGER NOT NULL DEFAULT 1,
	last_sync_at DATETIME,
	last_sync_status TEXT,
	last_sync_error TEXT,
	full_sync_status TEXT NOT NULL DEFAULT 'idle',
	full_sync_total INTEGER NOT NULL DEFAULT 0,
	full_sync_processed INTEGER NOT NULL DEFAULT 0,
	full_sync_new_count INTEGER NOT NULL DEFAULT 0,
	full_sync_started_at DATETIME,
	full_sync_finished_at DATETIME,
	full_sync_error TEXT,
	cleanup_enabled INTEGER NOT NULL DEFAULT 0,
	cleanup_retention_days INTEGER NOT NULL DEFAULT 90,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS mail_messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	account_id INTEGER NOT NULL,
	local_folder_id INTEGER,
	folder TEXT NOT NULL,
	imap_uid TEXT NOT NULL,
	message_id TEXT,
	subject TEXT,
	from_addr TEXT,
	to_addrs TEXT,
	cc_addrs TEXT,
	sent_at DATETIME,
	received_at DATETIME,
	has_attachments INTEGER NOT NULL DEFAULT 0,
	text_body_path TEXT,
	html_body_path TEXT,
	raw_path TEXT,
	search_text TEXT,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(account_id, folder, imap_uid),
	FOREIGN KEY(user_id) REFERENCES users(id),
	FOREIGN KEY(account_id) REFERENCES mail_accounts(id),
	FOREIGN KEY(local_folder_id) REFERENCES mail_folders(id)
);

CREATE TABLE IF NOT EXISTS mail_folders (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	color TEXT,
	sort_order INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user_id, name),
	FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS contacts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	email TEXT NOT NULL,
	email_key TEXT NOT NULL,
	display_name TEXT,
	nickname TEXT,
	phone TEXT,
	company TEXT,
	notes TEXT,
	source TEXT NOT NULL DEFAULT 'manual',
	first_seen_at DATETIME,
	last_seen_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user_id, email_key),
	FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS mail_rules (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	match_mode TEXT NOT NULL DEFAULT 'all',
	target_folder_id INTEGER NOT NULL,
	sort_order INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(user_id) REFERENCES users(id),
	FOREIGN KEY(target_folder_id) REFERENCES mail_folders(id)
);

CREATE TABLE IF NOT EXISTS mail_rule_conditions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	rule_id INTEGER NOT NULL,
	field TEXT NOT NULL,
	operator TEXT NOT NULL,
	value TEXT,
	FOREIGN KEY(rule_id) REFERENCES mail_rules(id)
);

CREATE TABLE IF NOT EXISTS mail_attachments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	message_id INTEGER NOT NULL,
	filename TEXT NOT NULL,
	content_type TEXT,
	content_id TEXT,
	inline INTEGER NOT NULL DEFAULT 0,
	size INTEGER NOT NULL DEFAULT 0,
	file_path TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(user_id) REFERENCES users(id),
	FOREIGN KEY(message_id) REFERENCES mail_messages(id)
);

CREATE TABLE IF NOT EXISTS mail_sync_jobs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	account_id INTEGER NOT NULL,
	trigger_type TEXT NOT NULL,
	status TEXT NOT NULL,
	started_at DATETIME,
	finished_at DATETIME,
	new_message_count INTEGER NOT NULL DEFAULT 0,
	error_message TEXT,
	FOREIGN KEY(user_id) REFERENCES users(id),
	FOREIGN KEY(account_id) REFERENCES mail_accounts(id)
);

CREATE INDEX IF NOT EXISTS idx_mail_messages_user_received ON mail_messages(user_id, received_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_mail_messages_account ON mail_messages(account_id, folder, imap_uid);
CREATE INDEX IF NOT EXISTS idx_mail_messages_user_sort ON mail_messages(user_id, COALESCE(sent_at, received_at, created_at) DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_mail_messages_user_folder_sort ON mail_messages(user_id, folder, COALESCE(sent_at, received_at, created_at) DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_mail_messages_user_account_sort ON mail_messages(user_id, account_id, COALESCE(sent_at, received_at, created_at) DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_mail_messages_user_attachment_sort ON mail_messages(user_id, has_attachments, COALESCE(sent_at, received_at, created_at) DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_mail_messages_user_local_folder_sort ON mail_messages(user_id, local_folder_id, COALESCE(sent_at, received_at, created_at) DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_mail_attachments_user_message ON mail_attachments(user_id, message_id, inline DESC, id ASC);
CREATE INDEX IF NOT EXISTS idx_contacts_user_updated ON contacts(user_id, updated_at DESC, id DESC);
`)
	if err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE mail_accounts ADD COLUMN provider TEXT NOT NULL DEFAULT 'custom'`,
		`ALTER TABLE mail_accounts ADD COLUMN auth_type TEXT NOT NULL DEFAULT 'password'`,
		`ALTER TABLE mail_accounts ADD COLUMN oauth_access_token_encrypted TEXT`,
		`ALTER TABLE mail_accounts ADD COLUMN oauth_refresh_token_encrypted TEXT`,
		`ALTER TABLE mail_accounts ADD COLUMN oauth_expires_at DATETIME`,
		`ALTER TABLE mail_attachments ADD COLUMN content_id TEXT`,
		`ALTER TABLE mail_attachments ADD COLUMN inline INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE mail_messages ADD COLUMN search_text TEXT`,
		`ALTER TABLE mail_messages ADD COLUMN local_folder_id INTEGER`,
		`ALTER TABLE mail_accounts ADD COLUMN full_sync_status TEXT NOT NULL DEFAULT 'idle'`,
		`ALTER TABLE mail_accounts ADD COLUMN full_sync_total INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE mail_accounts ADD COLUMN full_sync_processed INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE mail_accounts ADD COLUMN full_sync_new_count INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE mail_accounts ADD COLUMN full_sync_started_at DATETIME`,
		`ALTER TABLE mail_accounts ADD COLUMN full_sync_finished_at DATETIME`,
		`ALTER TABLE mail_accounts ADD COLUMN full_sync_error TEXT`,
		`ALTER TABLE mail_accounts ADD COLUMN cleanup_enabled INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE mail_accounts ADD COLUMN cleanup_retention_days INTEGER NOT NULL DEFAULT 90`,
		`ALTER TABLE users ADD COLUMN nickname TEXT`,
		`ALTER TABLE users ADD COLUMN avatar_path TEXT`,
		`ALTER TABLE users ADD COLUMN bio TEXT`,
		`ALTER TABLE mail_accounts ADD COLUMN sent_folder TEXT NOT NULL DEFAULT 'Sent'`,
		`ALTER TABLE mail_accounts ADD COLUMN smtp_host TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE mail_accounts ADD COLUMN smtp_port INTEGER NOT NULL DEFAULT 587`,
		`ALTER TABLE mail_accounts ADD COLUMN smtp_tls INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE mail_accounts ADD COLUMN smtp_starttls INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE mail_accounts ADD COLUMN smtp_username TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE mail_accounts ADD COLUMN smtp_password_encrypted TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE mail_accounts ADD COLUMN signature_html TEXT NOT NULL DEFAULT ''`,
	} {
		if _, alterErr := s.db.Exec(stmt); alterErr != nil && !strings.Contains(alterErr.Error(), "duplicate column name") {
			return alterErr
		}
	}
	return nil
}

func (s *Store) CreateUser(username, email, passwordHash string) (User, error) {
	result, err := s.db.Exec(
		`INSERT INTO users (username, email, password_hash) VALUES (?, ?, ?)`,
		username,
		email,
		passwordHash,
	)
	if err != nil {
		return User{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return User{}, err
	}

	return s.FindUserByID(id)
}

func (s *Store) FindUserByAccount(account string) (User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, email, password_hash, nickname, avatar_path, bio, created_at, updated_at FROM users WHERE username = ? OR email = ?`,
		account,
		account,
	)
	return scanUser(row)
}

func (s *Store) FindUserByID(id int64) (User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, email, password_hash, nickname, avatar_path, bio, created_at, updated_at FROM users WHERE id = ?`,
		id,
	)
	return scanUser(row)
}

func (s *Store) UpdateUserProfile(id int64, nickname, bio string) (User, error) {
	result, err := s.db.Exec(
		`UPDATE users SET nickname = ?, bio = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		nullIfEmpty(nickname),
		nullIfEmpty(bio),
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
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
}

var ErrNotFound = errors.New("not found")

func (s *Store) CreateMailAccount(account MailAccount) (MailAccount, error) {
	if strings.TrimSpace(account.Provider) == "" {
		account.Provider = "custom"
	}
	if strings.TrimSpace(account.AuthType) == "" {
		account.AuthType = "password"
	}
	result, err := s.db.Exec(
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

	id, err := result.LastInsertId()
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
	rows, err := s.db.Query(
		`SELECT id, user_id, provider, auth_type, display_name, email, imap_host, imap_port, imap_tls, imap_username,
			imap_password_encrypted, smtp_host, smtp_port, smtp_tls, smtp_starttls, smtp_username,
			smtp_password_encrypted, sent_folder, signature_html, oauth_access_token_encrypted, oauth_refresh_token_encrypted, oauth_expires_at,
			poll_interval_minutes, enabled, last_sync_at, last_sync_status,
			last_sync_error, full_sync_status, full_sync_total, full_sync_processed, full_sync_new_count,
			full_sync_started_at, full_sync_finished_at, full_sync_error, cleanup_enabled, cleanup_retention_days,
			created_at, updated_at
		FROM mail_accounts
		WHERE enabled = 1
			AND poll_interval_minutes > 0
			AND COALESCE(full_sync_status, 'idle') != 'running'
			AND (
				last_sync_at IS NULL
				OR datetime(last_sync_at, printf('+%d minutes', poll_interval_minutes)) <= CURRENT_TIMESTAMP
			)
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

func (s *Store) CreateSyncJob(userID, accountID int64, triggerType, status string) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO mail_sync_jobs (user_id, account_id, trigger_type, status, started_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		userID,
		accountID,
		triggerType,
		status,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) FinishSyncJob(id int64, status string, newMessageCount int, errMessage string) error {
	_, err := s.db.Exec(
		`UPDATE mail_sync_jobs
		SET status = ?, finished_at = CURRENT_TIMESTAMP, new_message_count = ?, error_message = ?
		WHERE id = ?`,
		status,
		newMessageCount,
		nullIfEmpty(errMessage),
		id,
	)
	return err
}

func (s *Store) InsertMailMessageIfNew(params CreateMailMessageParams) (MailMessage, bool, error) {
	result, err := s.db.Exec(
		`INSERT OR IGNORE INTO mail_messages (
			user_id, account_id, folder, imap_uid, message_id, subject, from_addr, to_addrs, cc_addrs,
			sent_at, received_at, has_attachments, text_body_path, html_body_path, raw_path, search_text
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
		`SELECT id, user_id, account_id, local_folder_id, folder, imap_uid, message_id, subject, from_addr, to_addrs, cc_addrs,
			sent_at, received_at, has_attachments, text_body_path, html_body_path, raw_path, search_text, created_at, updated_at
		FROM mail_messages
		WHERE user_id = ? AND account_id = ? AND folder = ? AND imap_uid = ?`,
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
	where := "WHERE user_id = ?"
	args := []any{query.UserID}
	if query.AccountID > 0 {
		where += " AND account_id = ?"
		args = append(args, query.AccountID)
	}
	if query.FolderID > 0 {
		where += " AND local_folder_id = ?"
		args = append(args, query.FolderID)
	}
	switch strings.TrimSpace(query.SystemFolder) {
	case "inbox":
		where += " AND folder = ?"
		args = append(args, "INBOX")
	case "sent":
		where += ` AND folder IN (
			SELECT COALESCE(NULLIF(TRIM(sent_folder), ''), 'Sent')
			FROM mail_accounts
			WHERE user_id = ?
		)`
		args = append(args, query.UserID)
	case "attachments":
		where += " AND has_attachments = 1"
	}
	if query.Keyword = strings.TrimSpace(query.Keyword); query.Keyword != "" {
		like := "%" + query.Keyword + "%"
		where += ` AND (
			COALESCE(subject, '') LIKE ?
			OR COALESCE(from_addr, '') LIKE ?
			OR COALESCE(to_addrs, '') LIKE ?
			OR COALESCE(cc_addrs, '') LIKE ?
			OR COALESCE(search_text, '') LIKE ?
		)`
		args = append(args, like, like, like, like, like)
	}
	if query.From = strings.TrimSpace(query.From); query.From != "" {
		where += " AND COALESCE(from_addr, '') LIKE ?"
		args = append(args, "%"+query.From+"%")
	}
	if query.Subject = strings.TrimSpace(query.Subject); query.Subject != "" {
		where += " AND COALESCE(subject, '') LIKE ?"
		args = append(args, "%"+query.Subject+"%")
	}
	if query.Body = strings.TrimSpace(query.Body); query.Body != "" {
		where += " AND COALESCE(search_text, '') LIKE ?"
		args = append(args, "%"+query.Body+"%")
	}
	if query.DateFrom.Valid {
		where += " AND COALESCE(sent_at, received_at, created_at) >= ?"
		args = append(args, query.DateFrom.Time)
	}
	if query.DateTo.Valid {
		where += " AND COALESCE(sent_at, received_at, created_at) < ?"
		args = append(args, query.DateTo.Time.AddDate(0, 0, 1))
	}
	if query.HasAttachments.Valid {
		where += " AND has_attachments = ?"
		args = append(args, boolToInt(query.HasAttachments.Bool))
	}

	countArgs := append([]any{}, args...)
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM mail_messages `+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	args = append(args, query.Limit, query.Offset)

	selectSQL := `SELECT id, user_id, account_id, local_folder_id, folder, imap_uid, message_id, subject, from_addr, to_addrs, cc_addrs,
			sent_at, received_at, has_attachments, text_body_path, html_body_path, raw_path, search_text, created_at, updated_at
		FROM mail_messages ` + where + `
		ORDER BY COALESCE(sent_at, received_at, created_at) DESC, id DESC
		LIMIT ? OFFSET ?`
	if query.SummaryOnly {
		selectSQL = `SELECT id, user_id, account_id, local_folder_id, folder, imap_uid, subject, from_addr, to_addrs,
			sent_at, received_at, has_attachments, created_at, updated_at
		FROM mail_messages ` + where + `
		ORDER BY COALESCE(sent_at, received_at, created_at) DESC, id DESC
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
		`SELECT id, user_id, account_id, local_folder_id, folder, imap_uid, message_id, subject, from_addr, to_addrs, cc_addrs,
			sent_at, received_at, has_attachments, text_body_path, html_body_path, raw_path, search_text, created_at, updated_at
		FROM mail_messages
		WHERE user_id = ? AND id = ?`,
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
		`SELECT id, user_id, account_id, local_folder_id, folder, imap_uid, message_id, subject, from_addr, to_addrs, cc_addrs,
			sent_at, received_at, has_attachments, text_body_path, html_body_path, raw_path, search_text, created_at, updated_at
		FROM mail_messages
		WHERE raw_path IS NOT NULL
		ORDER BY id ASC
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
			text_body_path = ?, html_body_path = ?, search_text = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		nullIfEmpty(params.MessageID),
		nullIfEmpty(params.Subject),
		nullIfEmpty(params.FromAddr),
		nullIfEmpty(params.ToAddrs),
		nullIfEmpty(params.CCAddrs),
		nullIfEmpty(params.TextBodyPath),
		nullIfEmpty(params.HTMLBodyPath),
		nullIfEmpty(params.SearchText),
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

func (s *Store) CreateMailFolder(params CreateMailFolderParams) (MailFolder, error) {
	result, err := s.db.Exec(
		`INSERT INTO mail_folders (user_id, name, color, sort_order) VALUES (?, ?, ?, ?)`,
		params.UserID,
		params.Name,
		nullIfEmpty(params.Color),
		params.SortOrder,
	)
	if err != nil {
		return MailFolder{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return MailFolder{}, err
	}
	return s.FindMailFolderByID(params.UserID, id)
}

func (s *Store) ListMailFolders(userID int64) ([]MailFolder, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, name, color, sort_order, created_at, updated_at
		FROM mail_folders
		WHERE user_id = ?
		ORDER BY sort_order ASC, name ASC, id ASC`,
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
		`SELECT id, user_id, name, color, sort_order, created_at, updated_at
		FROM mail_folders
		WHERE user_id = ? AND id = ?`,
		userID,
		id,
	)
	return scanMailFolder(row)
}

func (s *Store) DeleteMailFolder(userID, id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

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
	result, err := s.db.Exec(
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
	id, err := result.LastInsertId()
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
		`INSERT INTO contacts (
			user_id, email, email_key, display_name, source, first_seen_at, last_seen_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, email_key) DO UPDATE SET
			email = excluded.email,
			display_name = CASE
				WHEN contacts.display_name IS NULL OR TRIM(contacts.display_name) = '' THEN excluded.display_name
				ELSE contacts.display_name
			END,
			source = CASE
				WHEN contacts.source = 'auto' THEN excluded.source
				ELSE contacts.source
			END,
			last_seen_at = excluded.last_seen_at,
			updated_at = CURRENT_TIMESTAMP`,
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
		ORDER BY COALESCE(nickname, display_name, email) COLLATE NOCASE ASC, id ASC
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

func (s *Store) CreateMailRule(params CreateMailRuleParams) (MailRule, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return MailRule{}, err
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		`INSERT INTO mail_rules (user_id, name, enabled, match_mode, target_folder_id, sort_order)
		VALUES (?, ?, ?, ?, ?, ?)`,
		params.UserID,
		params.Name,
		boolToInt(params.Enabled),
		params.MatchMode,
		params.TargetFolderID,
		params.SortOrder,
	)
	if err != nil {
		return MailRule{}, err
	}
	ruleID, err := result.LastInsertId()
	if err != nil {
		return MailRule{}, err
	}
	for _, condition := range params.Conditions {
		if _, err := tx.Exec(
			`INSERT INTO mail_rule_conditions (rule_id, field, operator, value) VALUES (?, ?, ?, ?)`,
			ruleID,
			condition.Field,
			condition.Operator,
			condition.Value,
		); err != nil {
			return MailRule{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return MailRule{}, err
	}
	return s.FindMailRuleByID(params.UserID, ruleID)
}

func (s *Store) ListMailRules(userID int64, enabledOnly bool) ([]MailRule, error) {
	where := "WHERE user_id = ?"
	args := []any{userID}
	if enabledOnly {
		where += " AND enabled = 1"
	}
	rows, err := s.db.Query(
		`SELECT id, user_id, name, enabled, match_mode, target_folder_id, sort_order, created_at, updated_at
		FROM mail_rules `+where+`
		ORDER BY sort_order ASC, id ASC`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]MailRule, 0)
	for rows.Next() {
		rule, err := scanMailRule(rows)
		if err != nil {
			return nil, err
		}
		conditions, err := s.ListMailRuleConditions(rule.ID)
		if err != nil {
			return nil, err
		}
		rule.Conditions = conditions
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}

func (s *Store) FindMailRuleByID(userID, id int64) (MailRule, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, name, enabled, match_mode, target_folder_id, sort_order, created_at, updated_at
		FROM mail_rules
		WHERE user_id = ? AND id = ?`,
		userID,
		id,
	)
	rule, err := scanMailRule(row)
	if err != nil {
		return MailRule{}, err
	}
	conditions, err := s.ListMailRuleConditions(rule.ID)
	if err != nil {
		return MailRule{}, err
	}
	rule.Conditions = conditions
	return rule, nil
}

func (s *Store) UpdateMailRule(userID, id int64, params CreateMailRuleParams) (MailRule, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return MailRule{}, err
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		`UPDATE mail_rules
		SET name = ?, enabled = ?, match_mode = ?, target_folder_id = ?, sort_order = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		params.Name,
		boolToInt(params.Enabled),
		params.MatchMode,
		params.TargetFolderID,
		params.SortOrder,
		userID,
		id,
	)
	if err != nil {
		return MailRule{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return MailRule{}, err
	}
	if count == 0 {
		return MailRule{}, ErrNotFound
	}
	if _, err := tx.Exec(`DELETE FROM mail_rule_conditions WHERE rule_id = ?`, id); err != nil {
		return MailRule{}, err
	}
	for _, condition := range params.Conditions {
		if _, err := tx.Exec(
			`INSERT INTO mail_rule_conditions (rule_id, field, operator, value) VALUES (?, ?, ?, ?)`,
			id,
			condition.Field,
			condition.Operator,
			condition.Value,
		); err != nil {
			return MailRule{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return MailRule{}, err
	}
	return s.FindMailRuleByID(userID, id)
}

func (s *Store) DeleteMailRule(userID, id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(`DELETE FROM mail_rule_conditions WHERE rule_id IN (SELECT id FROM mail_rules WHERE user_id = ? AND id = ?)`, userID, id)
	if err != nil {
		return err
	}
	_, _ = result.RowsAffected()

	result, err = tx.Exec(`DELETE FROM mail_rules WHERE user_id = ? AND id = ?`, userID, id)
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

func (s *Store) ListMailRuleConditions(ruleID int64) ([]MailRuleCondition, error) {
	rows, err := s.db.Query(
		`SELECT id, rule_id, field, operator, value
		FROM mail_rule_conditions
		WHERE rule_id = ?
		ORDER BY id ASC`,
		ruleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	conditions := make([]MailRuleCondition, 0)
	for rows.Next() {
		condition, err := scanMailRuleCondition(rows)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, condition)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return conditions, nil
}

func (s *Store) CreateMailAttachment(params CreateMailAttachmentParams) (MailAttachment, error) {
	if strings.TrimSpace(params.Filename) == "" {
		params.Filename = "attachment"
	}
	result, err := s.db.Exec(
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
	id, err := result.LastInsertId()
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

func scanMailMessage(scanner interface {
	Scan(dest ...any) error
}) (MailMessage, error) {
	var message MailMessage
	var hasAttachments int
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
	return message, nil
}

func scanMailMessageSummary(scanner interface {
	Scan(dest ...any) error
}) (MailMessage, error) {
	var message MailMessage
	var hasAttachments int
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
	return message, nil
}

func scanMailFolder(scanner interface {
	Scan(dest ...any) error
}) (MailFolder, error) {
	var folder MailFolder
	err := scanner.Scan(
		&folder.ID,
		&folder.UserID,
		&folder.Name,
		&folder.Color,
		&folder.SortOrder,
		&folder.CreatedAt,
		&folder.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return MailFolder{}, ErrNotFound
	}
	if err != nil {
		return MailFolder{}, err
	}
	return folder, nil
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

func scanMailRule(scanner interface {
	Scan(dest ...any) error
}) (MailRule, error) {
	var rule MailRule
	var enabled int
	err := scanner.Scan(
		&rule.ID,
		&rule.UserID,
		&rule.Name,
		&enabled,
		&rule.MatchMode,
		&rule.TargetFolderID,
		&rule.SortOrder,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return MailRule{}, ErrNotFound
	}
	if err != nil {
		return MailRule{}, err
	}
	rule.Enabled = enabled == 1
	return rule, nil
}

func scanMailRuleCondition(scanner interface {
	Scan(dest ...any) error
}) (MailRuleCondition, error) {
	var condition MailRuleCondition
	err := scanner.Scan(
		&condition.ID,
		&condition.RuleID,
		&condition.Field,
		&condition.Operator,
		&condition.Value,
	)
	if err != nil {
		return MailRuleCondition{}, err
	}
	return condition, nil
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

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func normalizeRetentionDays(value int) int {
	if value <= 0 {
		return 90
	}
	return value
}

func normalizeSMTPPort(value int) int {
	if value <= 0 {
		return 587
	}
	return value
}

func normalizeSentFolder(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Sent"
	}
	return value
}

func normalizeEmailKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullTimeValue(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time
}

func nullStringValue(value sql.NullString) any {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	return value.String
}

func nullInt64Value(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}
