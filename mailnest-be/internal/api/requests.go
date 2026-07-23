package api

type registerRequest struct {
	Username      string `json:"username"`
	Email         string `json:"email"`
	Password      string `json:"password"`
	CaptchaID     string `json:"captchaId"`
	CaptchaAnswer string `json:"captchaAnswer"`
}

type loginRequest struct {
	Account       string `json:"account"`
	Password      string `json:"password"`
	CaptchaID     string `json:"captchaId"`
	CaptchaAnswer string `json:"captchaAnswer"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

type updateProfileRequest struct {
	Nickname string `json:"nickname"`
	Bio      string `json:"bio"`
	UITheme  string `json:"uiTheme"`
}

type createMailAccountRequest struct {
	DisplayName          string `json:"displayName"`
	Email                string `json:"email"`
	IMAPHost             string `json:"imapHost"`
	IMAPPort             int    `json:"imapPort"`
	IMAPTLS              bool   `json:"imapTls"`
	IMAPUsername         string `json:"imapUsername"`
	IMAPPassword         string `json:"imapPassword"`
	SMTPHost             string `json:"smtpHost"`
	SMTPPort             int    `json:"smtpPort"`
	SMTPTLS              bool   `json:"smtpTls"`
	SMTPStartTLS         bool   `json:"smtpStartTls"`
	SMTPUsername         string `json:"smtpUsername"`
	SMTPPassword         string `json:"smtpPassword"`
	SMTPUseIMAPPassword  bool   `json:"smtpUseImapPassword"`
	SentFolder           string `json:"sentFolder"`
	SignatureHTML        string `json:"signatureHtml"`
	PollIntervalMinutes  int    `json:"pollIntervalMinutes"`
	Enabled              bool   `json:"enabled"`
	CleanupEnabled       bool   `json:"cleanupEnabled"`
	CleanupRetentionDays int    `json:"cleanupRetentionDays"`
}

type completeMicrosoftOAuthRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

type createMailFolderRequest struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sortOrder"`
}

type contactRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Nickname    string `json:"nickname"`
	Phone       string `json:"phone"`
	Company     string `json:"company"`
	Notes       string `json:"notes"`
}

type assignMessageFolderRequest struct {
	FolderID string `json:"folderId"`
}

type sendMessageRequest struct {
	AccountID            string   `json:"accountId"`
	To                   []string `json:"to"`
	CC                   []string `json:"cc"`
	BCC                  []string `json:"bcc"`
	Subject              string   `json:"subject"`
	TextBody             string   `json:"textBody"`
	HTMLBody             string   `json:"htmlBody"`
	ComposeMode          string   `json:"composeMode"`
	SourceMessageID      string   `json:"sourceMessageId"`
	ForwardAttachmentIDs []string `json:"forwardAttachmentIds"`
}

type createMailRuleRequest struct {
	Name           string                    `json:"name"`
	Enabled        bool                      `json:"enabled"`
	MatchMode      string                    `json:"matchMode"`
	Priority       int                       `json:"priority"`
	StopOnMatch    bool                      `json:"stopOnMatch"`
	ActionType     string                    `json:"actionType"`
	TargetFolderID string                    `json:"targetFolderId"`
	SortOrder      int                       `json:"sortOrder"`
	Conditions     []createMailRuleCondition `json:"conditions"`
}

type createMailRuleCondition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type applyMailRulesRequest struct {
	Scope string `json:"scope"`
}

type previewMailRuleRequest struct {
	Name           string                    `json:"name"`
	Enabled        bool                      `json:"enabled"`
	MatchMode      string                    `json:"matchMode"`
	Priority       int                       `json:"priority"`
	StopOnMatch    bool                      `json:"stopOnMatch"`
	ActionType     string                    `json:"actionType"`
	TargetFolderID string                    `json:"targetFolderId"`
	SortOrder      int                       `json:"sortOrder"`
	Conditions     []createMailRuleCondition `json:"conditions"`
	Limit          int                       `json:"limit"`
}

type messageBatchActionRequest struct {
	MessageIDs []string `json:"messageIds"`
	Action     string   `json:"action"`
	FolderID   string   `json:"folderId"`
}

type messageBatchPreviewRequest struct {
	MessageIDs []string `json:"messageIds"`
}

type updateUserEnabledRequest struct {
	Enabled bool `json:"enabled"`
}
