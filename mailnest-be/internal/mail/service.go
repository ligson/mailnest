package mail

import (
	"database/sql"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"mailnest-be/internal/crypto"
	"mailnest-be/internal/oauth"
	"mailnest-be/internal/storage"
)

type Service struct {
	store            *storage.Store
	fetcher          Fetcher
	exchanger        oauth.MicrosoftExchanger
	dataDir          string
	credentialSecret string
}

type SyncResult struct {
	JobID           int64
	NewMessageCount int
}

func NewService(store *storage.Store, fetcher Fetcher, exchanger oauth.MicrosoftExchanger, dataDir, credentialSecret string) *Service {
	if fetcher == nil {
		fetcher = NewIMAPFetcher()
	}
	return &Service{
		store:            store,
		fetcher:          fetcher,
		exchanger:        exchanger,
		dataDir:          dataDir,
		credentialSecret: credentialSecret,
	}
}

func (s *Service) TestConnection(userID, accountID int64) error {
	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return err
	}
	config, err := s.accountConfig(account)
	if err != nil {
		return err
	}
	if err := s.fetcher.TestConnection(config); err != nil {
		_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "failed", err.Error())
		return err
	}
	return s.store.UpdateMailAccountSyncStatus(userID, accountID, "connection_ok", "")
}

func (s *Service) SyncInbox(userID, accountID int64) (SyncResult, error) {
	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return SyncResult{}, err
	}

	jobID, err := s.store.CreateSyncJob(userID, accountID, "manual", "running")
	if err != nil {
		return SyncResult{}, err
	}

	config, err := s.accountConfig(account)
	if err != nil {
		_ = s.store.FinishSyncJob(jobID, "failed", 0, err.Error())
		_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "failed", err.Error())
		return SyncResult{JobID: jobID}, err
	}

	messages, err := s.fetcher.FetchInbox(config)
	if err != nil {
		_ = s.store.FinishSyncJob(jobID, "failed", 0, err.Error())
		_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "failed", err.Error())
		return SyncResult{JobID: jobID}, err
	}

	newCount := 0
	for _, fetched := range messages {
		inserted, err := s.saveMessage(userID, account.ID, fetched)
		if err != nil {
			_ = s.store.FinishSyncJob(jobID, "failed", newCount, err.Error())
			_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "failed", err.Error())
			return SyncResult{JobID: jobID, NewMessageCount: newCount}, err
		}
		if inserted {
			newCount++
		}
	}

	_ = s.store.FinishSyncJob(jobID, "success", newCount, "")
	_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "success", "")
	return SyncResult{JobID: jobID, NewMessageCount: newCount}, nil
}

func (s *Service) accountConfig(account storage.MailAccount) (AccountConfig, error) {
	password := ""
	accessToken := ""
	if account.AuthType == "oauth2" {
		token, err := s.oauthAccessToken(account)
		if err != nil {
			return AccountConfig{}, err
		}
		accessToken = token
	} else if strings.TrimSpace(account.IMAPPasswordEncoded) != "" {
		decrypted, err := crypto.DecryptString(account.IMAPPasswordEncoded, s.credentialSecret)
		if err != nil {
			return AccountConfig{}, err
		}
		password = decrypted
	}
	return AccountConfig{
		Email:       account.Email,
		Host:        account.IMAPHost,
		Port:        account.IMAPPort,
		TLS:         account.IMAPTLS,
		Username:    account.IMAPUsername,
		Password:    password,
		AccessToken: accessToken,
		AuthType:    account.AuthType,
		Provider:    account.Provider,
		Folder:      "INBOX",
	}, nil
}

func (s *Service) oauthAccessToken(account storage.MailAccount) (string, error) {
	if account.OAuthAccessToken.Valid && (!account.OAuthExpiresAt.Valid || time.Until(account.OAuthExpiresAt.Time) > 2*time.Minute) {
		return crypto.DecryptString(account.OAuthAccessToken.String, s.credentialSecret)
	}
	if s.exchanger == nil || !account.OAuthRefreshToken.Valid {
		if account.OAuthAccessToken.Valid {
			return crypto.DecryptString(account.OAuthAccessToken.String, s.credentialSecret)
		}
		return "", fmt.Errorf("OAuth token 不存在，请重新授权")
	}
	refreshToken, err := crypto.DecryptString(account.OAuthRefreshToken.String, s.credentialSecret)
	if err != nil {
		return "", err
	}
	token, err := s.exchanger.Refresh(refreshToken)
	if err != nil {
		return "", err
	}
	encryptedAccess, err := crypto.EncryptString(token.AccessToken, s.credentialSecret)
	if err != nil {
		return "", err
	}
	encryptedRefresh := account.OAuthRefreshToken.String
	if strings.TrimSpace(token.RefreshToken) != "" {
		encryptedRefresh, err = crypto.EncryptString(token.RefreshToken, s.credentialSecret)
		if err != nil {
			return "", err
		}
	}
	if err := s.store.UpdateMailAccountOAuthTokens(account.UserID, account.ID, encryptedAccess, encryptedRefresh, token.ExpiresAt); err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

func (s *Service) saveMessage(userID, accountID int64, fetched FetchedMessage) (bool, error) {
	uid := strings.TrimSpace(fetched.UID)
	if uid == "" {
		uid = strings.TrimSpace(fetched.MessageID)
	}
	if uid == "" {
		uid = fmt.Sprintf("generated-%d", time.Now().UnixNano())
	}

	messageDir := filepath.Join(s.dataDir, "users", fmt.Sprint(userID), "accounts", fmt.Sprint(accountID), "messages", safePath(uid))
	if err := os.MkdirAll(messageDir, 0o755); err != nil {
		return false, err
	}

	rawPath, err := writeContent(messageDir, "raw.eml", fetched.RawContent)
	if err != nil {
		return false, err
	}
	textPath, err := writeContent(messageDir, "body.txt", fetched.TextBody)
	if err != nil {
		return false, err
	}
	htmlPath, err := writeContent(messageDir, "body.html", fetched.HTMLBody)
	if err != nil {
		return false, err
	}

	sentAt := parseTime(fetched.SentAt)
	receivedAt := sql.NullTime{Time: time.Now(), Valid: true}
	toAddrs := strings.Join(fetched.To, ", ")
	ccAddrs := strings.Join(fetched.CC, ", ")

	_, inserted, err := s.store.InsertMailMessageIfNew(storage.CreateMailMessageParams{
		UserID:         userID,
		AccountID:      accountID,
		Folder:         "INBOX",
		IMAPUID:        uid,
		MessageID:      fetched.MessageID,
		Subject:        fetched.Subject,
		FromAddr:       fetched.From,
		ToAddrs:        toAddrs,
		CCAddrs:        ccAddrs,
		SentAt:         sentAt,
		ReceivedAt:     receivedAt,
		HasAttachments: len(fetched.Attachments) > 0,
		TextBodyPath:   textPath,
		HTMLBodyPath:   htmlPath,
		RawPath:        rawPath,
		SearchText:     buildSearchText(fetched, toAddrs, ccAddrs),
	})
	if err != nil {
		return false, err
	}
	if !inserted {
		if len(fetched.Attachments) > 0 {
			message, err := s.store.FindMailMessageByUID(userID, accountID, "INBOX", uid)
			if err != nil {
				return false, err
			}
			existingAttachments, err := s.store.ListMailAttachments(userID, message.ID)
			if err != nil {
				return false, err
			}
			if len(existingAttachments) == 0 {
				for index, attachment := range fetched.Attachments {
					if err := s.saveAttachment(userID, message.ID, messageDir, index, attachment); err != nil {
						return false, err
					}
				}
				if err := s.store.UpdateMailMessageHasAttachments(userID, message.ID, true); err != nil {
					return false, err
				}
			}
		}
		return false, nil
	}
	message, err := s.store.FindMailMessageByUID(userID, accountID, "INBOX", uid)
	if err != nil {
		return false, err
	}
	for index, attachment := range fetched.Attachments {
		if err := s.saveAttachment(userID, message.ID, messageDir, index, attachment); err != nil {
			return false, err
		}
	}
	if _, err := s.ApplyRulesToMessage(userID, message, false); err != nil {
		return false, err
	}
	return inserted, nil
}

func writeContent(dir, name, content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", nil
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Service) saveAttachment(userID, messageID int64, messageDir string, index int, attachment FetchedAttachment) error {
	if len(attachment.Data) == 0 {
		return nil
	}
	attachmentDir := filepath.Join(messageDir, "attachments")
	if err := os.MkdirAll(attachmentDir, 0o755); err != nil {
		return err
	}
	filename := strings.TrimSpace(attachment.Filename)
	if filename == "" {
		filename = fmt.Sprintf("attachment-%d", index+1)
	}
	filePath := filepath.Join(attachmentDir, fmt.Sprintf("%03d-%s", index+1, safePath(filename)))
	if err := os.WriteFile(filePath, attachment.Data, 0o600); err != nil {
		return err
	}
	_, err := s.store.CreateMailAttachment(storage.CreateMailAttachmentParams{
		UserID:      userID,
		MessageID:   messageID,
		Filename:    filename,
		ContentType: attachment.ContentType,
		ContentID:   attachment.ContentID,
		Inline:      attachment.Inline,
		Size:        int64(len(attachment.Data)),
		FilePath:    filePath,
	})
	return err
}

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

func buildSearchText(fetched FetchedMessage, toAddrs, ccAddrs string) string {
	parts := []string{
		fetched.Subject,
		fetched.From,
		toAddrs,
		ccAddrs,
		fetched.TextBody,
		stripHTMLTags(fetched.HTMLBody),
	}
	return strings.Join(parts, "\n")
}

func stripHTMLTags(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	withoutTags := htmlTagPattern.ReplaceAllString(value, " ")
	return html.UnescapeString(withoutTags)
}

func parseTime(value string) sql.NullTime {
	if strings.TrimSpace(value) == "" {
		return sql.NullTime{}
	}
	for _, layout := range []string{time.RFC3339, time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return sql.NullTime{Time: parsed, Valid: true}
		}
	}
	return sql.NullTime{}
}

func safePath(value string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "..", "_")
	return replacer.Replace(value)
}
