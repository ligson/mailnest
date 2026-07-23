package storage

import (
	"fmt"
	"strings"
	"time"
)

func (s *Store) migrateGORM() error {
	if s.db.dialect == dialectMySQL && s.db.gormDB.Migrator().HasTable(&userModel{}) {
		return s.createSupplementalIndexes()
	}
	if err := s.db.gormDB.AutoMigrate(
		&userModel{},
		&mailAccountModel{},
		&mailFolderModel{},
		&mailMessageModel{},
		&contactModel{},
		&mailRuleModel{},
		&mailRuleConditionModel{},
		&mailAttachmentModel{},
		&mailSyncJobModel{},
		&mailMessageStateModel{},
		&mailSyncJobEventModel{},
	); err != nil {
		return fmt.Errorf("gorm automigrate %s: %w", s.db.dialect, err)
	}
	return s.createSupplementalIndexes()
}

func (s *Store) migrateExistingSQLite() error {
	models := []struct {
		table string
		model any
	}{
		{table: "users", model: &userModel{}},
		{table: "mail_accounts", model: &mailAccountModel{}},
		{table: "mail_folders", model: &mailFolderModel{}},
		{table: "mail_messages", model: &mailMessageModel{}},
		{table: "contacts", model: &contactModel{}},
		{table: "mail_rules", model: &mailRuleModel{}},
		{table: "mail_rule_conditions", model: &mailRuleConditionModel{}},
		{table: "mail_attachments", model: &mailAttachmentModel{}},
		{table: "mail_sync_jobs", model: &mailSyncJobModel{}},
		{table: "mail_message_states", model: &mailMessageStateModel{}},
		{table: "mail_sync_job_events", model: &mailSyncJobEventModel{}},
	}
	for _, item := range models {
		exists, err := s.sqliteTableExists(item.table)
		if err != nil {
			return err
		}
		if !exists {
			if err := s.db.gormDB.AutoMigrate(item.model); err != nil {
				return fmt.Errorf("gorm automigrate missing sqlite table %s: %w", item.table, err)
			}
		}
	}
	for _, column := range sqliteExistingColumnStatements() {
		if err := s.addSQLiteColumnIfMissing(column.table, column.name, column.definition); err != nil {
			return err
		}
	}
	if err := s.createSQLiteExistingIndexes(); err != nil {
		return err
	}
	return s.createSupplementalIndexes()
}

func (s *Store) sqliteTableExists(table string) (bool, error) {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) sqliteColumnExists(table, column string) (bool, error) {
	if !safeSQLiteIdentifier(table) || !safeSQLiteIdentifier(column) {
		return false, fmt.Errorf("unsafe sqlite identifier %q.%q", table, column)
	}
	rows, err := s.db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if strings.EqualFold(name, column) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Store) addSQLiteColumnIfMissing(table, column, definition string) error {
	exists, err := s.sqliteColumnExists(table, column)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if _, err := s.db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s`, table, definition)); err != nil {
		return fmt.Errorf("add sqlite column %s.%s: %w", table, column, err)
	}
	return nil
}

func (s *Store) createSQLiteExistingIndexes() error {
	for _, stmt := range sqliteExistingIndexStatements() {
		if _, err := s.db.Exec(stmt); err != nil {
			if isSchemaAlreadyExistsError(err) {
				continue
			}
			return fmt.Errorf("create sqlite existing index: %w", err)
		}
	}
	return nil
}

func safeSQLiteIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

type sqliteColumnStatement struct {
	table      string
	name       string
	definition string
}

func sqliteExistingColumnStatements() []sqliteColumnStatement {
	return []sqliteColumnStatement{
		{table: "mail_accounts", name: "provider", definition: `provider TEXT NOT NULL DEFAULT 'custom'`},
		{table: "mail_accounts", name: "auth_type", definition: `auth_type TEXT NOT NULL DEFAULT 'password'`},
		{table: "mail_accounts", name: "oauth_access_token_encrypted", definition: `oauth_access_token_encrypted TEXT`},
		{table: "mail_accounts", name: "oauth_refresh_token_encrypted", definition: `oauth_refresh_token_encrypted TEXT`},
		{table: "mail_accounts", name: "oauth_expires_at", definition: `oauth_expires_at DATETIME`},
		{table: "mail_accounts", name: "full_sync_status", definition: `full_sync_status TEXT NOT NULL DEFAULT 'idle'`},
		{table: "mail_accounts", name: "full_sync_total", definition: `full_sync_total INTEGER NOT NULL DEFAULT 0`},
		{table: "mail_accounts", name: "full_sync_processed", definition: `full_sync_processed INTEGER NOT NULL DEFAULT 0`},
		{table: "mail_accounts", name: "full_sync_new_count", definition: `full_sync_new_count INTEGER NOT NULL DEFAULT 0`},
		{table: "mail_accounts", name: "full_sync_started_at", definition: `full_sync_started_at DATETIME`},
		{table: "mail_accounts", name: "full_sync_finished_at", definition: `full_sync_finished_at DATETIME`},
		{table: "mail_accounts", name: "full_sync_error", definition: `full_sync_error TEXT`},
		{table: "mail_accounts", name: "cleanup_enabled", definition: `cleanup_enabled INTEGER NOT NULL DEFAULT 0`},
		{table: "mail_accounts", name: "cleanup_retention_days", definition: `cleanup_retention_days INTEGER NOT NULL DEFAULT 90`},
		{table: "mail_accounts", name: "sent_folder", definition: `sent_folder TEXT NOT NULL DEFAULT 'Sent'`},
		{table: "mail_accounts", name: "smtp_host", definition: `smtp_host TEXT NOT NULL DEFAULT ''`},
		{table: "mail_accounts", name: "smtp_port", definition: `smtp_port INTEGER NOT NULL DEFAULT 587`},
		{table: "mail_accounts", name: "smtp_tls", definition: `smtp_tls INTEGER NOT NULL DEFAULT 0`},
		{table: "mail_accounts", name: "smtp_starttls", definition: `smtp_starttls INTEGER NOT NULL DEFAULT 1`},
		{table: "mail_accounts", name: "smtp_username", definition: `smtp_username TEXT NOT NULL DEFAULT ''`},
		{table: "mail_accounts", name: "smtp_password_encrypted", definition: `smtp_password_encrypted TEXT NOT NULL DEFAULT ''`},
		{table: "mail_accounts", name: "signature_html", definition: `signature_html TEXT NOT NULL DEFAULT ''`},
		{table: "mail_attachments", name: "content_id", definition: `content_id TEXT`},
		{table: "mail_attachments", name: "inline", definition: `inline INTEGER NOT NULL DEFAULT 0`},
		{table: "mail_messages", name: "search_text", definition: `search_text TEXT`},
		{table: "mail_messages", name: "local_folder_id", definition: `local_folder_id INTEGER`},
		{table: "mail_messages", name: "in_reply_to", definition: `in_reply_to TEXT`},
		{table: "mail_messages", name: "references_header", definition: `references_header TEXT`},
		{table: "mail_messages", name: "source_message_id", definition: `source_message_id INTEGER`},
		{table: "mail_messages", name: "compose_mode", definition: `compose_mode TEXT`},
		{table: "users", name: "nickname", definition: `nickname TEXT`},
		{table: "users", name: "avatar_path", definition: `avatar_path TEXT`},
		{table: "users", name: "bio", definition: `bio TEXT`},
		{table: "users", name: "ui_theme", definition: `ui_theme TEXT NOT NULL DEFAULT 'forest'`},
		{table: "mail_rules", name: "priority", definition: `priority INTEGER NOT NULL DEFAULT 0`},
		{table: "mail_rules", name: "stop_on_match", definition: `stop_on_match INTEGER NOT NULL DEFAULT 1`},
		{table: "mail_rules", name: "action_type", definition: `action_type TEXT NOT NULL DEFAULT 'move_folder'`},
		{table: "mail_message_states", name: "is_spam", definition: `is_spam INTEGER NOT NULL DEFAULT 0`},
		{table: "mail_message_states", name: "spam_at", definition: `spam_at DATETIME`},
	}
}

func sqliteExistingIndexStatements() []string {
	return []string{
		`CREATE INDEX IF NOT EXISTS idx_mail_messages_user_received ON mail_messages(user_id, received_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_messages_account ON mail_messages(account_id, folder, imap_uid)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_messages_user_sort ON mail_messages(user_id, COALESCE(sent_at, received_at, created_at) DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_messages_user_folder_sort ON mail_messages(user_id, folder, COALESCE(sent_at, received_at, created_at) DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_messages_user_account_sort ON mail_messages(user_id, account_id, COALESCE(sent_at, received_at, created_at) DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_messages_user_attachment_sort ON mail_messages(user_id, has_attachments, COALESCE(sent_at, received_at, created_at) DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_messages_user_local_folder_sort ON mail_messages(user_id, local_folder_id, COALESCE(sent_at, received_at, created_at) DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_attachments_user_message ON mail_attachments(user_id, message_id, inline DESC, id ASC)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_attachments_user_filename ON mail_attachments(user_id, filename)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_attachments_user_content_type ON mail_attachments(user_id, content_type)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_attachments_user_created ON mail_attachments(user_id, created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_attachments_user_inline ON mail_attachments(user_id, inline, id)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_message_states_user_message ON mail_message_states(user_id, message_id)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_message_states_user_deleted ON mail_message_states(user_id, deleted_at, message_id)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_message_states_user_spam ON mail_message_states(user_id, is_spam, message_id)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_sync_jobs_user_account ON mail_sync_jobs(user_id, account_id, started_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_sync_job_events_job_created ON mail_sync_job_events(job_id, created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_user_updated ON contacts(user_id, updated_at DESC, id DESC)`,
	}
}

func (s *Store) createSupplementalIndexes() error {
	if err := s.ensureDialectColumnTypes(); err != nil {
		return err
	}
	for _, stmt := range supplementalIndexStatements(s.db.dialect) {
		if _, err := s.db.Exec(stmt); err != nil {
			if isSchemaAlreadyExistsError(err) {
				continue
			}
			return fmt.Errorf("create supplemental index %s: %w", s.db.dialect, err)
		}
	}
	return nil
}

func (s *Store) ensureDialectColumnTypes() error {
	if s.db.dialect != dialectMySQL {
		return nil
	}
	_, err := s.db.Exec(`ALTER TABLE mail_messages MODIFY COLUMN search_text LONGTEXT NULL`)
	if err != nil {
		return fmt.Errorf("ensure mysql long text columns: %w", err)
	}
	return nil
}

func isSchemaAlreadyExistsError(err error) bool {
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "already exists") ||
		strings.Contains(text, "duplicate key name") ||
		strings.Contains(text, "duplicate key") ||
		strings.Contains(text, "relation") && strings.Contains(text, "already exists")
}

func supplementalIndexStatements(dialect dbDialect) []string {
	switch dialect {
	case dialectMySQL:
		return []string{
			`CREATE INDEX idx_mail_messages_user_sort ON mail_messages(user_id, sent_at DESC, received_at DESC, created_at DESC, id DESC)`,
			`CREATE INDEX idx_mail_messages_user_folder_sort ON mail_messages(user_id, folder, sent_at DESC, received_at DESC, created_at DESC, id DESC)`,
			`CREATE INDEX idx_mail_messages_user_account_sort ON mail_messages(user_id, account_id, sent_at DESC, received_at DESC, created_at DESC, id DESC)`,
			`CREATE INDEX idx_mail_messages_user_attachment_sort ON mail_messages(user_id, has_attachments, sent_at DESC, received_at DESC, created_at DESC, id DESC)`,
			`CREATE INDEX idx_mail_messages_user_local_folder_sort ON mail_messages(user_id, local_folder_id, sent_at DESC, received_at DESC, created_at DESC, id DESC)`,
		}
	case dialectPostgres:
		return []string{
			`CREATE INDEX IF NOT EXISTS idx_mail_messages_user_sort ON mail_messages(user_id, COALESCE(sent_at, received_at, created_at) DESC, id DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_mail_messages_user_folder_sort ON mail_messages(user_id, folder, COALESCE(sent_at, received_at, created_at) DESC, id DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_mail_messages_user_account_sort ON mail_messages(user_id, account_id, COALESCE(sent_at, received_at, created_at) DESC, id DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_mail_messages_user_attachment_sort ON mail_messages(user_id, has_attachments, COALESCE(sent_at, received_at, created_at) DESC, id DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_mail_messages_user_local_folder_sort ON mail_messages(user_id, local_folder_id, COALESCE(sent_at, received_at, created_at) DESC, id DESC)`,
		}
	default:
		return []string{
			`CREATE INDEX IF NOT EXISTS idx_mail_messages_user_sort ON mail_messages(user_id, COALESCE(sent_at, received_at, created_at) DESC, id DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_mail_messages_user_folder_sort ON mail_messages(user_id, folder, COALESCE(sent_at, received_at, created_at) DESC, id DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_mail_messages_user_account_sort ON mail_messages(user_id, account_id, COALESCE(sent_at, received_at, created_at) DESC, id DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_mail_messages_user_attachment_sort ON mail_messages(user_id, has_attachments, COALESCE(sent_at, received_at, created_at) DESC, id DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_mail_messages_user_local_folder_sort ON mail_messages(user_id, local_folder_id, COALESCE(sent_at, received_at, created_at) DESC, id DESC)`,
		}
	}
}

type userModel struct {
	ID           int64     `gorm:"primaryKey;autoIncrement;column:id"`
	Username     string    `gorm:"column:username;size:255;not null;uniqueIndex"`
	Email        string    `gorm:"column:email;size:255;not null;uniqueIndex"`
	PasswordHash string    `gorm:"column:password_hash;size:255;not null"`
	Nickname     *string   `gorm:"column:nickname;size:255"`
	AvatarPath   *string   `gorm:"column:avatar_path;type:text"`
	Bio          *string   `gorm:"column:bio;type:text"`
	UITheme      string    `gorm:"column:ui_theme;size:32;not null;default:forest"`
	CreatedAt    time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP"`
}

func (userModel) TableName() string { return "users" }

type mailAccountModel struct {
	ID                         int64      `gorm:"primaryKey;autoIncrement;column:id"`
	UserID                     int64      `gorm:"column:user_id;not null;index"`
	Provider                   string     `gorm:"column:provider;size:64;not null;default:custom"`
	AuthType                   string     `gorm:"column:auth_type;size:64;not null;default:password"`
	DisplayName                string     `gorm:"column:display_name;size:255;not null"`
	Email                      string     `gorm:"column:email;size:255;not null"`
	IMAPHost                   string     `gorm:"column:imap_host;size:255;not null"`
	IMAPPort                   int        `gorm:"column:imap_port;not null"`
	IMAPTLS                    int        `gorm:"column:imap_tls;not null;default:1"`
	IMAPUsername               string     `gorm:"column:imap_username;size:255;not null"`
	IMAPPasswordEncrypted      string     `gorm:"column:imap_password_encrypted;type:text;not null"`
	SMTPHost                   string     `gorm:"column:smtp_host;size:255;not null;default:''"`
	SMTPPort                   int        `gorm:"column:smtp_port;not null;default:587"`
	SMTPTLS                    int        `gorm:"column:smtp_tls;not null;default:0"`
	SMTPStartTLS               int        `gorm:"column:smtp_starttls;not null;default:1"`
	SMTPUsername               string     `gorm:"column:smtp_username;size:255;not null;default:''"`
	SMTPPasswordEncrypted      string     `gorm:"column:smtp_password_encrypted;type:text;not null"`
	SentFolder                 string     `gorm:"column:sent_folder;size:255;not null;default:Sent"`
	SignatureHTML              string     `gorm:"column:signature_html;type:text;not null"`
	OAuthAccessTokenEncrypted  *string    `gorm:"column:oauth_access_token_encrypted;type:text"`
	OAuthRefreshTokenEncrypted *string    `gorm:"column:oauth_refresh_token_encrypted;type:text"`
	OAuthExpiresAt             *time.Time `gorm:"column:oauth_expires_at"`
	PollIntervalMinutes        int        `gorm:"column:poll_interval_minutes;not null;default:10"`
	Enabled                    int        `gorm:"column:enabled;not null;default:1"`
	LastSyncAt                 *time.Time `gorm:"column:last_sync_at"`
	LastSyncStatus             *string    `gorm:"column:last_sync_status;size:64"`
	LastSyncError              *string    `gorm:"column:last_sync_error;type:text"`
	FullSyncStatus             string     `gorm:"column:full_sync_status;size:64;not null;default:idle"`
	FullSyncTotal              int        `gorm:"column:full_sync_total;not null;default:0"`
	FullSyncProcessed          int        `gorm:"column:full_sync_processed;not null;default:0"`
	FullSyncNewCount           int        `gorm:"column:full_sync_new_count;not null;default:0"`
	FullSyncStartedAt          *time.Time `gorm:"column:full_sync_started_at"`
	FullSyncFinishedAt         *time.Time `gorm:"column:full_sync_finished_at"`
	FullSyncError              *string    `gorm:"column:full_sync_error;type:text"`
	CleanupEnabled             int        `gorm:"column:cleanup_enabled;not null;default:0"`
	CleanupRetentionDays       int        `gorm:"column:cleanup_retention_days;not null;default:90"`
	CreatedAt                  time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt                  time.Time  `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP"`
}

func (mailAccountModel) TableName() string { return "mail_accounts" }

type mailFolderModel struct {
	ID        int64     `gorm:"primaryKey;autoIncrement;column:id"`
	UserID    int64     `gorm:"column:user_id;not null;uniqueIndex:idx_mail_folders_user_name,priority:1"`
	Name      string    `gorm:"column:name;size:255;not null;uniqueIndex:idx_mail_folders_user_name,priority:2"`
	Color     *string   `gorm:"column:color;size:32"`
	SortOrder int       `gorm:"column:sort_order;not null;default:0"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP"`
}

func (mailFolderModel) TableName() string { return "mail_folders" }

type mailMessageModel struct {
	ID              int64      `gorm:"primaryKey;autoIncrement;column:id;index:idx_mail_messages_user_received,priority:3,sort:desc"`
	UserID          int64      `gorm:"column:user_id;not null;index:idx_mail_messages_user_received,priority:1"`
	AccountID       int64      `gorm:"column:account_id;not null;uniqueIndex:idx_mail_messages_account_folder_uid,priority:1;index:idx_mail_messages_account,priority:1"`
	LocalFolderID   *int64     `gorm:"column:local_folder_id"`
	Folder          string     `gorm:"column:folder;size:255;not null;uniqueIndex:idx_mail_messages_account_folder_uid,priority:2;index:idx_mail_messages_account,priority:2"`
	IMAPUID         string     `gorm:"column:imap_uid;size:255;not null;uniqueIndex:idx_mail_messages_account_folder_uid,priority:3;index:idx_mail_messages_account,priority:3"`
	MessageID       *string    `gorm:"column:message_id;size:512"`
	Subject         *string    `gorm:"column:subject;type:text"`
	FromAddr        *string    `gorm:"column:from_addr;type:text"`
	ToAddrs         *string    `gorm:"column:to_addrs;type:text"`
	CCAddrs         *string    `gorm:"column:cc_addrs;type:text"`
	SentAt          *time.Time `gorm:"column:sent_at"`
	ReceivedAt      *time.Time `gorm:"column:received_at;index:idx_mail_messages_user_received,priority:2,sort:desc"`
	HasAttachments  int        `gorm:"column:has_attachments;not null;default:0"`
	TextBodyPath    *string    `gorm:"column:text_body_path;type:text"`
	HTMLBodyPath    *string    `gorm:"column:html_body_path;type:text"`
	RawPath         *string    `gorm:"column:raw_path;type:text"`
	SearchText      *string    `gorm:"column:search_text;type:text"`
	InReplyTo       *string    `gorm:"column:in_reply_to;type:text"`
	References      *string    `gorm:"column:references_header;type:text"`
	SourceMessageID *int64     `gorm:"column:source_message_id"`
	ComposeMode     *string    `gorm:"column:compose_mode;size:64"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP"`
}

func (mailMessageModel) TableName() string { return "mail_messages" }

type contactModel struct {
	ID          int64      `gorm:"primaryKey;autoIncrement;column:id;index:idx_contacts_user_updated,priority:3,sort:desc"`
	UserID      int64      `gorm:"column:user_id;not null;uniqueIndex:idx_contacts_user_email_key,priority:1;index:idx_contacts_user_updated,priority:1"`
	Email       string     `gorm:"column:email;size:255;not null"`
	EmailKey    string     `gorm:"column:email_key;size:255;not null;uniqueIndex:idx_contacts_user_email_key,priority:2"`
	DisplayName *string    `gorm:"column:display_name;size:255"`
	Nickname    *string    `gorm:"column:nickname;size:255"`
	Phone       *string    `gorm:"column:phone;size:64"`
	Company     *string    `gorm:"column:company;size:255"`
	Notes       *string    `gorm:"column:notes;type:text"`
	Source      string     `gorm:"column:source;size:64;not null;default:manual"`
	FirstSeenAt *time.Time `gorm:"column:first_seen_at"`
	LastSeenAt  *time.Time `gorm:"column:last_seen_at"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;index:idx_contacts_user_updated,priority:2,sort:desc"`
}

func (contactModel) TableName() string { return "contacts" }

type mailRuleModel struct {
	ID             int64     `gorm:"primaryKey;autoIncrement;column:id"`
	UserID         int64     `gorm:"column:user_id;not null;index"`
	Name           string    `gorm:"column:name;size:255;not null"`
	Enabled        int       `gorm:"column:enabled;not null;default:1"`
	MatchMode      string    `gorm:"column:match_mode;size:32;not null;default:all"`
	Priority       int       `gorm:"column:priority;not null;default:0"`
	StopOnMatch    int       `gorm:"column:stop_on_match;not null;default:1"`
	ActionType     string    `gorm:"column:action_type;size:64;not null;default:move_folder"`
	TargetFolderID int64     `gorm:"column:target_folder_id;not null;index"`
	SortOrder      int       `gorm:"column:sort_order;not null;default:0"`
	CreatedAt      time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP"`
}

func (mailRuleModel) TableName() string { return "mail_rules" }

type mailRuleConditionModel struct {
	ID       int64   `gorm:"primaryKey;autoIncrement;column:id"`
	RuleID   int64   `gorm:"column:rule_id;not null;index"`
	Field    string  `gorm:"column:field;size:64;not null"`
	Operator string  `gorm:"column:operator;size:64;not null"`
	Value    *string `gorm:"column:value;type:text"`
}

func (mailRuleConditionModel) TableName() string { return "mail_rule_conditions" }

type mailAttachmentModel struct {
	ID          int64     `gorm:"primaryKey;autoIncrement;column:id;index:idx_mail_attachments_user_message,priority:4;index:idx_mail_attachments_user_created,priority:3,sort:desc;index:idx_mail_attachments_user_inline,priority:3"`
	UserID      int64     `gorm:"column:user_id;not null;index:idx_mail_attachments_user_message,priority:1;index:idx_mail_attachments_user_filename,priority:1;index:idx_mail_attachments_user_content_type,priority:1;index:idx_mail_attachments_user_created,priority:1;index:idx_mail_attachments_user_inline,priority:1"`
	MessageID   int64     `gorm:"column:message_id;not null;index:idx_mail_attachments_user_message,priority:2"`
	Filename    string    `gorm:"column:filename;size:512;not null;index:idx_mail_attachments_user_filename,priority:2"`
	ContentType *string   `gorm:"column:content_type;size:255;index:idx_mail_attachments_user_content_type,priority:2"`
	ContentID   *string   `gorm:"column:content_id;size:512"`
	Inline      int       `gorm:"column:inline;not null;default:0;index:idx_mail_attachments_user_message,priority:3,sort:desc;index:idx_mail_attachments_user_inline,priority:2"`
	Size        int64     `gorm:"column:size;not null;default:0"`
	FilePath    string    `gorm:"column:file_path;type:text;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP;index:idx_mail_attachments_user_created,priority:2,sort:desc"`
}

func (mailAttachmentModel) TableName() string { return "mail_attachments" }

type mailSyncJobModel struct {
	ID              int64      `gorm:"primaryKey;autoIncrement;column:id;index:idx_mail_sync_jobs_user_account,priority:4,sort:desc"`
	UserID          int64      `gorm:"column:user_id;not null;index:idx_mail_sync_jobs_user_account,priority:1"`
	AccountID       int64      `gorm:"column:account_id;not null;index:idx_mail_sync_jobs_user_account,priority:2"`
	TriggerType     string     `gorm:"column:trigger_type;size:64;not null"`
	Status          string     `gorm:"column:status;size:64;not null"`
	StartedAt       *time.Time `gorm:"column:started_at;index:idx_mail_sync_jobs_user_account,priority:3,sort:desc"`
	FinishedAt      *time.Time `gorm:"column:finished_at"`
	NewMessageCount int        `gorm:"column:new_message_count;not null;default:0"`
	ErrorMessage    *string    `gorm:"column:error_message;type:text"`
}

func (mailSyncJobModel) TableName() string { return "mail_sync_jobs" }

type mailMessageStateModel struct {
	ID         int64      `gorm:"primaryKey;autoIncrement;column:id"`
	UserID     int64      `gorm:"column:user_id;not null;uniqueIndex:idx_mail_message_states_user_message_unique,priority:1;index:idx_mail_message_states_user_message,priority:1;index:idx_mail_message_states_user_deleted,priority:1;index:idx_mail_message_states_user_spam,priority:1"`
	MessageID  int64      `gorm:"column:message_id;not null;uniqueIndex:idx_mail_message_states_user_message_unique,priority:2;index:idx_mail_message_states_user_message,priority:2;index:idx_mail_message_states_user_deleted,priority:3;index:idx_mail_message_states_user_spam,priority:3"`
	IsRead     int        `gorm:"column:is_read;not null;default:0"`
	ReadAt     *time.Time `gorm:"column:read_at"`
	Starred    int        `gorm:"column:starred;not null;default:0"`
	IsSpam     int        `gorm:"column:is_spam;not null;default:0;index:idx_mail_message_states_user_spam,priority:2"`
	SpamAt     *time.Time `gorm:"column:spam_at"`
	ArchivedAt *time.Time `gorm:"column:archived_at"`
	DeletedAt  *time.Time `gorm:"column:deleted_at;index:idx_mail_message_states_user_deleted,priority:2"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP"`
}

func (mailMessageStateModel) TableName() string { return "mail_message_states" }

type mailSyncJobEventModel struct {
	ID         int64     `gorm:"primaryKey;autoIncrement;column:id;index:idx_mail_sync_job_events_job_created,priority:3,sort:desc"`
	JobID      int64     `gorm:"column:job_id;not null;index:idx_mail_sync_job_events_job_created,priority:1"`
	Level      string    `gorm:"column:level;size:32;not null"`
	Phase      string    `gorm:"column:phase;size:64;not null"`
	Message    string    `gorm:"column:message;type:text;not null"`
	DetailJSON *string   `gorm:"column:detail_json;type:text"`
	CreatedAt  time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP;index:idx_mail_sync_job_events_job_created,priority:2,sort:desc"`
}

func (mailSyncJobEventModel) TableName() string { return "mail_sync_job_events" }
