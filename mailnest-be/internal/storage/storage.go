package storage

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

type User struct {
	ID           int64
	Username     string
	Email        string
	PasswordHash string
	Nickname     sql.NullString
	AvatarPath   sql.NullString
	Bio          sql.NullString
	UITheme      string
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
	ID              int64
	UserID          int64
	AccountID       int64
	LocalFolderID   sql.NullInt64
	Folder          string
	IMAPUID         string
	MessageID       sql.NullString
	Subject         sql.NullString
	FromAddr        sql.NullString
	ToAddrs         sql.NullString
	CCAddrs         sql.NullString
	SentAt          sql.NullTime
	ReceivedAt      sql.NullTime
	HasAttachments  bool
	TextBodyPath    sql.NullString
	HTMLBodyPath    sql.NullString
	RawPath         sql.NullString
	SearchText      sql.NullString
	InReplyTo       sql.NullString
	References      sql.NullString
	SourceMessageID sql.NullInt64
	ComposeMode     sql.NullString
	IsRead          bool
	Starred         bool
	IsSpam          bool
	SpamAt          sql.NullTime
	DeletedAt       sql.NullTime
	CreatedAt       time.Time
	UpdatedAt       time.Time
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
	RuleCount int
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
	Priority       int
	StopOnMatch    bool
	ActionType     string
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
	UserID          int64
	AccountID       int64
	Folder          string
	IMAPUID         string
	MessageID       string
	Subject         string
	FromAddr        string
	ToAddrs         string
	CCAddrs         string
	SentAt          sql.NullTime
	ReceivedAt      sql.NullTime
	HasAttachments  bool
	TextBodyPath    string
	HTMLBodyPath    string
	RawPath         string
	SearchText      string
	InReplyTo       string
	References      string
	SourceMessageID sql.NullInt64
	ComposeMode     string
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
	InReplyTo    string
	References   string
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
	IncludeDeleted bool
	OnlyDeleted    bool
	IsRead         sql.NullBool
	Starred        sql.NullBool
	IsSpam         sql.NullBool
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

var ErrMailFolderHasRules = errors.New("mail folder has rules")

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
	Priority       int
	StopOnMatch    bool
	ActionType     string
	TargetFolderID int64
	SortOrder      int
	Conditions     []CreateMailRuleConditionParams
}

type CreateMailRuleConditionParams struct {
	Field    string
	Operator string
	Value    string
}

type MessageBatchActionParams struct {
	UserID     int64
	MessageIDs []int64
	Action     string
	FolderID   sql.NullInt64
}

type MessageBatchActionResult struct {
	MatchedCount int
	ChangedCount int
	SkippedCount int
}

type MessageBatchPreview struct {
	Total        int
	ReadCount    int
	UnreadCount  int
	StarredCount int
	SpamCount    int
	DeletedCount int
	FolderCounts []MessageBatchFolderCount
}

type MessageBatchFolderCount struct {
	FolderID int64
	Name     string
	Count    int
}

type ListAttachmentsQuery struct {
	UserID      int64
	Keyword     string
	ContentType string
	AccountID   int64
	FolderID    int64
	Inline      sql.NullBool
	DateFrom    sql.NullTime
	DateTo      sql.NullTime
	Limit       int
	Offset      int
}

type AttachmentListItem struct {
	Attachment     MailAttachment
	AccountID      int64
	LocalFolderID  sql.NullInt64
	MessageSubject sql.NullString
	MessageFrom    sql.NullString
	MessageTime    sql.NullTime
}

type MailSyncJob struct {
	ID              int64
	UserID          int64
	AccountID       int64
	TriggerType     string
	Status          string
	StartedAt       sql.NullTime
	FinishedAt      sql.NullTime
	NewMessageCount int
	ErrorMessage    sql.NullString
}

type ListSyncJobsQuery struct {
	UserID    int64
	AccountID int64
	Limit     int
	Offset    int
}

type MailSyncJobEvent struct {
	ID         int64
	JobID      int64
	Level      string
	Phase      string
	Message    string
	DetailJSON sql.NullString
	CreatedAt  time.Time
}

type ListSyncJobEventsQuery struct {
	UserID int64
	JobID  int64
	Level  string
	Limit  int
	Offset int
}

type Store struct {
	db *database
}

func Open(path string) (*Store, error) {
	return OpenWithOptions(DatabaseOptions{
		Driver: "sqlite",
		Path:   path,
	})
}

func OpenWithOptions(options DatabaseOptions) (*Store, error) {
	db, err := openDatabase(options)
	if err != nil {
		return nil, err
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	return s.migrateGORM()
}

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

var ErrNotFound = errors.New("not found")

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

func (s *Store) CreateSyncJob(userID, accountID int64, triggerType, status string) (int64, error) {
	return s.db.insertAndGetID(
		`INSERT INTO mail_sync_jobs (user_id, account_id, trigger_type, status, started_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		userID,
		accountID,
		triggerType,
		status,
	)
}

func (s *Store) ListSyncJobs(query ListSyncJobsQuery) ([]MailSyncJob, int, error) {
	where := "WHERE user_id = ?"
	args := []any{query.UserID}
	if query.AccountID > 0 {
		where += " AND account_id = ?"
		args = append(args, query.AccountID)
	}
	countArgs := append([]any{}, args...)
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM mail_sync_jobs `+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 50
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	args = append(args, query.Limit, query.Offset)
	rows, err := s.db.Query(
		`SELECT id, user_id, account_id, trigger_type, status, started_at, finished_at, new_message_count, error_message
		FROM mail_sync_jobs `+where+`
		ORDER BY started_at DESC, id DESC
		LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]MailSyncJob, 0)
	for rows.Next() {
		var job MailSyncJob
		if err := rows.Scan(&job.ID, &job.UserID, &job.AccountID, &job.TriggerType, &job.Status, &job.StartedAt, &job.FinishedAt, &job.NewMessageCount, &job.ErrorMessage); err != nil {
			return nil, 0, err
		}
		items = append(items, job)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Store) FindSyncJobByID(userID, id int64) (MailSyncJob, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, account_id, trigger_type, status, started_at, finished_at, new_message_count, error_message
		FROM mail_sync_jobs
		WHERE user_id = ? AND id = ?`,
		userID,
		id,
	)
	var job MailSyncJob
	if err := row.Scan(&job.ID, &job.UserID, &job.AccountID, &job.TriggerType, &job.Status, &job.StartedAt, &job.FinishedAt, &job.NewMessageCount, &job.ErrorMessage); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MailSyncJob{}, ErrNotFound
		}
		return MailSyncJob{}, err
	}
	return job, nil
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

func (s *Store) CreateSyncJobEvent(jobID int64, level, phase, message string, detailJSON string) error {
	_, err := s.db.Exec(
		`INSERT INTO mail_sync_job_events (job_id, level, phase, message, detail_json) VALUES (?, ?, ?, ?, ?)`,
		jobID,
		level,
		phase,
		message,
		nullIfEmpty(detailJSON),
	)
	return err
}

func (s *Store) ListSyncJobEvents(query ListSyncJobEventsQuery) ([]MailSyncJobEvent, int, error) {
	where := "WHERE j.user_id = ? AND e.job_id = ?"
	args := []any{query.UserID, query.JobID}
	if query.Level = strings.TrimSpace(query.Level); query.Level != "" {
		where += " AND e.level = ?"
		args = append(args, query.Level)
	}
	countArgs := append([]any{}, args...)
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM mail_sync_job_events e JOIN mail_sync_jobs j ON j.id = e.job_id `+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 100
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	args = append(args, query.Limit, query.Offset)
	rows, err := s.db.Query(
		`SELECT e.id, e.job_id, e.level, e.phase, e.message, e.detail_json, e.created_at
		FROM mail_sync_job_events e
		JOIN mail_sync_jobs j ON j.id = e.job_id `+where+`
		ORDER BY e.created_at DESC, e.id DESC
		LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]MailSyncJobEvent, 0)
	for rows.Next() {
		var event MailSyncJobEvent
		if err := rows.Scan(&event.ID, &event.JobID, &event.Level, &event.Phase, &event.Message, &event.DetailJSON, &event.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, event)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

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

func (s *Store) CreateMailRule(params CreateMailRuleParams) (MailRule, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return MailRule{}, err
	}
	defer tx.Rollback()

	ruleID, err := tx.insertAndGetID(
		`INSERT INTO mail_rules (user_id, name, enabled, match_mode, priority, stop_on_match, action_type, target_folder_id, sort_order)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		params.UserID,
		params.Name,
		boolToInt(params.Enabled),
		params.MatchMode,
		params.Priority,
		boolToInt(params.StopOnMatch),
		normalizeRuleActionType(params.ActionType),
		params.TargetFolderID,
		params.SortOrder,
	)
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
		`SELECT id, user_id, name, enabled, match_mode, priority, stop_on_match, action_type, target_folder_id, sort_order, created_at, updated_at
		FROM mail_rules `+where+`
		ORDER BY priority ASC, sort_order ASC, id ASC`,
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
		`SELECT id, user_id, name, enabled, match_mode, priority, stop_on_match, action_type, target_folder_id, sort_order, created_at, updated_at
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
		SET name = ?, enabled = ?, match_mode = ?, priority = ?, stop_on_match = ?, action_type = ?, target_folder_id = ?, sort_order = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND id = ?`,
		params.Name,
		boolToInt(params.Enabled),
		params.MatchMode,
		params.Priority,
		boolToInt(params.StopOnMatch),
		normalizeRuleActionType(params.ActionType),
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
	var stopOnMatch int
	err := scanner.Scan(
		&rule.ID,
		&rule.UserID,
		&rule.Name,
		&enabled,
		&rule.MatchMode,
		&rule.Priority,
		&stopOnMatch,
		&rule.ActionType,
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
	rule.StopOnMatch = stopOnMatch == 1
	rule.ActionType = normalizeRuleActionType(rule.ActionType)
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

func normalizeRuleActionType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "mark_read", "mark_unread", "star", "unstar", "mark_spam", "unmark_spam", "move_folder", "delete", "restore":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "move_folder"
	}
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
