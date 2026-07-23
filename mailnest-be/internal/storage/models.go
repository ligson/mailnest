package storage

import (
	"database/sql"
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
	IsAdmin      bool
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type AdminUserSummary struct {
	User             User
	MailAccountCount int
	MessageCount     int
	AttachmentCount  int
	AttachmentBytes  int64
	ContactCount     int
	FolderCount      int
	RuleCount        int
	LastMessageAt    sql.NullTime
	LastSyncAt       sql.NullTime
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
	ThreadID        sql.NullInt64
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

type MailThread struct {
	ID                int64
	UserID            int64
	AccountID         int64
	RootMessageID     sql.NullInt64
	Subject           string
	NormalizedSubject string
	MessageCount      int
	UnreadCount       int
	HasAttachments    bool
	LastMessageAt     sql.NullTime
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type MailThreadListItem struct {
	Thread        MailThread
	LatestMessage MailMessage
	Participants  []string
}

type MailDraft struct {
	ID                       int64
	UserID                   int64
	AccountID                int64
	ComposeMode              string
	SourceMessageID          sql.NullInt64
	ToAddrsJSON              string
	CCAddrsJSON              string
	BCCAddrsJSON             string
	Subject                  string
	TextBody                 string
	HTMLBody                 string
	ForwardAttachmentIDsJSON string
	LocalAttachmentNamesJSON string
	CreatedAt                time.Time
	UpdatedAt                time.Time
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
	HitCount       int
	LastHitAt      sql.NullTime
	LastResult     sql.NullString
}

type MailRuleCondition struct {
	ID       int64
	RuleID   int64
	Field    string
	Operator string
	Value    string
}

type MailRuleLog struct {
	ID                    int64
	UserID                int64
	RuleID                sql.NullInt64
	RuleName              string
	MessageID             int64
	MessageSubject        sql.NullString
	Matched               bool
	ActionType            string
	TargetFolderID        sql.NullInt64
	TriggerType           string
	ConditionSnapshotJSON string
	ResultStatus          string
	ResultMessage         string
	CreatedAt             time.Time
}

type CreateMailMessageParams struct {
	UserID          int64
	AccountID       int64
	ThreadID        sql.NullInt64
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

type CreateMailThreadParams struct {
	UserID            int64
	AccountID         int64
	RootMessageID     sql.NullInt64
	Subject           string
	NormalizedSubject string
	LastMessageAt     sql.NullTime
	HasAttachments    bool
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

type SaveMailDraftParams struct {
	ID                       int64
	UserID                   int64
	AccountID                int64
	ComposeMode              string
	SourceMessageID          sql.NullInt64
	ToAddrsJSON              string
	CCAddrsJSON              string
	BCCAddrsJSON             string
	Subject                  string
	TextBody                 string
	HTMLBody                 string
	ForwardAttachmentIDsJSON string
	LocalAttachmentNamesJSON string
}

type ListMailDraftsQuery struct {
	UserID int64
	Limit  int
	Offset int
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

type ListMailThreadsQuery struct {
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
}

type RebuildThreadsParams struct {
	UserID    int64
	AccountID int64
	Scope     string
}

type RebuildThreadsResult struct {
	ProcessedCount int
	ThreadCount    int
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

type CreateMailRuleLogParams struct {
	UserID                int64
	RuleID                int64
	RuleName              string
	MessageID             int64
	Matched               bool
	ActionType            string
	TargetFolderID        int64
	TriggerType           string
	ConditionSnapshotJSON string
	ResultStatus          string
	ResultMessage         string
}

type ListMailRuleLogsQuery struct {
	UserID       int64
	MessageID    int64
	RuleID       int64
	ResultStatus string
	TriggerType  string
	Limit        int
	Offset       int
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
