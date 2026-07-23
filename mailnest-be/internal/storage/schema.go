package storage

import (
	"fmt"
	"strings"
	"time"
)

func (s *Store) migrateGORM() error {
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

func (s *Store) createSupplementalIndexes() error {
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
