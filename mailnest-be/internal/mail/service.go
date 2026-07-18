package mail

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"log"
	netmail "net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"mailnest-be/internal/crypto"
	"mailnest-be/internal/oauth"
	"mailnest-be/internal/storage"
)

type Service struct {
	store            *storage.Store
	fetcher          Fetcher
	sender           Sender
	exchanger        oauth.MicrosoftExchanger
	dataDir          string
	credentialSecret string
	fullSyncMu       sync.Mutex
	fullSyncCancels  map[string]chan struct{}
	inboxSyncMu      sync.Mutex
	inboxSyncs       map[string]struct{}
}

type SyncResult struct {
	JobID           int64
	NewMessageCount int
	Warnings        []string
}

type FullSyncStatus struct {
	Status         string
	Total          int
	Processed      int
	NewCount       int
	StartedAt      sql.NullTime
	FinishedAt     sql.NullTime
	Error          sql.NullString
	CleanupEnabled bool
	RetentionDays  int
}

type AutoSyncOptions struct {
	CheckInterval  time.Duration
	BatchLimit     int
	MaxConcurrent  int
	RunImmediately bool
}

const fullSyncBatchSize = 50
const parsedContentRepairLimit = 5000
const defaultAutoSyncCheckInterval = time.Minute
const defaultAutoSyncBatchLimit = 20
const defaultAutoSyncMaxConcurrent = 2

var ErrSyncAlreadyRunning = errors.New("邮箱账号正在收取中")

func NewService(store *storage.Store, fetcher Fetcher, exchanger oauth.MicrosoftExchanger, dataDir, credentialSecret string) *Service {
	if fetcher == nil {
		fetcher = NewIMAPFetcher()
	}
	return NewServiceWithSender(store, fetcher, &SMTPSender{}, exchanger, dataDir, credentialSecret)
}

func NewServiceWithSender(store *storage.Store, fetcher Fetcher, sender Sender, exchanger oauth.MicrosoftExchanger, dataDir, credentialSecret string) *Service {
	if fetcher == nil {
		fetcher = NewIMAPFetcher()
	}
	if sender == nil {
		sender = &SMTPSender{}
	}
	return &Service{
		store:            store,
		fetcher:          fetcher,
		sender:           sender,
		exchanger:        exchanger,
		dataDir:          dataDir,
		credentialSecret: credentialSecret,
		fullSyncCancels:  make(map[string]chan struct{}),
		inboxSyncs:       make(map[string]struct{}),
	}
}

func (s *Service) SendMessage(userID int64, accountID int64, message OutgoingMessage) (storage.MailMessage, error) {
	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return storage.MailMessage{}, err
	}
	config, err := s.smtpConfig(account)
	if err != nil {
		return storage.MailMessage{}, err
	}
	message.From = account.Email
	message.FromName = account.DisplayName
	result, err := s.sender.Send(config, message)
	if err != nil {
		return storage.MailMessage{}, err
	}

	fetched := FetchedMessage{
		UID:        "sent-" + safePath(strings.Trim(result.MessageID, "<>")),
		MessageID:  result.MessageID,
		Subject:    strings.TrimSpace(message.Subject),
		From:       (&netmail.Address{Name: account.DisplayName, Address: account.Email}).String(),
		To:         nonEmptyStrings(message.To),
		CC:         nonEmptyStrings(message.CC),
		SentAt:     result.SentAt.Format(time.RFC3339),
		TextBody:   message.TextBody,
		HTMLBody:   message.HTMLBody,
		RawContent: result.Raw,
	}
	for _, attachment := range message.Attachments {
		fetched.Attachments = append(fetched.Attachments, FetchedAttachment{
			Filename:    attachment.Filename,
			ContentType: attachment.ContentType,
			Data:        attachment.Data,
		})
	}
	if _, err := s.saveMessage(userID, accountID, normalizeSentFolder(account.SentFolder), fetched); err != nil {
		return storage.MailMessage{}, err
	}
	if err := s.upsertBCCContacts(userID, message.BCC, result.SentAt); err != nil {
		log.Printf("upsert bcc contacts user=%d account=%d: %v", userID, accountID, err)
	}
	return s.store.FindMailMessageByUID(userID, accountID, normalizeSentFolder(account.SentFolder), fetched.UID)
}

func (s *Service) StartAutoSyncScheduler(ctx context.Context, options AutoSyncOptions) {
	options = normalizeAutoSyncOptions(options)
	sem := make(chan struct{}, options.MaxConcurrent)
	go func() {
		log.Printf("mail auto sync scheduler started, interval=%s, batchLimit=%d, maxConcurrent=%d", options.CheckInterval, options.BatchLimit, options.MaxConcurrent)
		if options.RunImmediately {
			s.dispatchDueAutoSyncs(ctx, options, sem)
		}
		ticker := time.NewTicker(options.CheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Printf("mail auto sync scheduler stopped")
				return
			case <-ticker.C:
				s.dispatchDueAutoSyncs(ctx, options, sem)
			}
		}
	}()
}

func normalizeAutoSyncOptions(options AutoSyncOptions) AutoSyncOptions {
	if options.CheckInterval <= 0 {
		options.CheckInterval = defaultAutoSyncCheckInterval
	}
	if options.BatchLimit <= 0 || options.BatchLimit > 100 {
		options.BatchLimit = defaultAutoSyncBatchLimit
	}
	if options.MaxConcurrent <= 0 || options.MaxConcurrent > 10 {
		options.MaxConcurrent = defaultAutoSyncMaxConcurrent
	}
	return options
}

func (s *Service) dispatchDueAutoSyncs(ctx context.Context, options AutoSyncOptions, sem chan struct{}) {
	accounts, err := s.store.ListDueMailAccounts(options.BatchLimit)
	if err != nil {
		log.Printf("mail auto sync list due accounts failed: %v", err)
		return
	}
	for _, account := range accounts {
		if ctx.Err() != nil {
			return
		}
		if !s.tryRegisterInboxSync(account.UserID, account.ID) {
			continue
		}
		select {
		case sem <- struct{}{}:
		default:
			s.unregisterInboxSync(account.UserID, account.ID)
			return
		}
		go func(account storage.MailAccount) {
			defer func() {
				<-sem
				s.unregisterInboxSync(account.UserID, account.ID)
			}()
			result, err := s.syncInbox(account, "auto")
			if err != nil {
				log.Printf("mail auto sync failed account=%d user=%d: %v", account.ID, account.UserID, err)
				return
			}
			log.Printf("mail auto sync finished account=%d user=%d new=%d", account.ID, account.UserID, result.NewMessageCount)
		}(account)
	}
}

func (s *Service) RepairStoredParsedMessages() error {
	messages, err := s.store.ListMailMessagesWithRawContent(parsedContentRepairLimit)
	if err != nil {
		return err
	}
	for _, message := range messages {
		if !message.RawPath.Valid || strings.TrimSpace(message.RawPath.String) == "" {
			continue
		}
		raw, err := os.ReadFile(message.RawPath.String)
		if err != nil {
			continue
		}
		parsed := fetchedMessageFromRaw(raw)
		currentText := readContentFile(nullableStringValue(message.TextBodyPath))
		currentHTML := readContentFile(nullableStringValue(message.HTMLBodyPath))
		if !messageNeedsParsedRepair(message, currentText, currentHTML, parsed) {
			continue
		}
		textPath := nullableStringValue(message.TextBodyPath)
		htmlPath := nullableStringValue(message.HTMLBodyPath)
		messageDir := filepath.Dir(message.RawPath.String)
		if strings.TrimSpace(parsed.TextBody) != "" {
			if path, err := writeContent(messageDir, "body.txt", parsed.TextBody); err == nil {
				textPath = path
			}
		}
		if strings.TrimSpace(parsed.HTMLBody) != "" {
			if path, err := writeContent(messageDir, "body.html", parsed.HTMLBody); err == nil {
				htmlPath = path
			}
		}
		toAddrs := strings.Join(parsed.To, ", ")
		ccAddrs := strings.Join(parsed.CC, ", ")
		if err := s.store.UpdateMailMessageParsedContent(storage.UpdateMailMessageContentParams{
			UserID:       message.UserID,
			ID:           message.ID,
			MessageID:    parsed.MessageID,
			Subject:      valueOrExisting(parsed.Subject, nullableStringValue(message.Subject)),
			FromAddr:     valueOrExisting(parsed.From, nullableStringValue(message.FromAddr)),
			ToAddrs:      valueOrExisting(toAddrs, nullableStringValue(message.ToAddrs)),
			CCAddrs:      valueOrExisting(ccAddrs, nullableStringValue(message.CCAddrs)),
			TextBodyPath: textPath,
			HTMLBodyPath: htmlPath,
			SearchText:   buildSearchText(parsed, toAddrs, ccAddrs),
		}); err != nil {
			return err
		}
	}
	return nil
}

func messageNeedsParsedRepair(message storage.MailMessage, currentText, currentHTML string, parsed FetchedMessage) bool {
	if containsEncodedWord(nullableStringValue(message.Subject)) ||
		containsEncodedWord(nullableStringValue(message.FromAddr)) ||
		containsEncodedWord(nullableStringValue(message.ToAddrs)) ||
		containsEncodedWord(nullableStringValue(message.CCAddrs)) {
		return true
	}
	if looksLikeMIMEBody(currentText) || looksLikeMIMEBody(currentHTML) {
		return true
	}
	if strings.TrimSpace(currentText) == "" && strings.TrimSpace(parsed.TextBody) != "" {
		return true
	}
	if strings.TrimSpace(currentHTML) == "" && strings.TrimSpace(parsed.HTMLBody) != "" {
		return true
	}
	return false
}

func readContentFile(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(content)
}

func containsEncodedWord(value string) bool {
	return strings.Contains(strings.ToLower(value), "=?")
}

func looksLikeMIMEBody(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(trimmed, "--") &&
		(strings.Contains(lower, "content-type:") || strings.Contains(lower, "content-transfer-encoding:"))
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

func (s *Service) ListFolders(userID, accountID int64) ([]FolderInfo, error) {
	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return nil, err
	}
	config, err := s.accountConfig(account)
	if err != nil {
		return nil, err
	}
	return s.fetcher.ListFolders(config)
}

func (s *Service) SyncInbox(userID, accountID int64) (SyncResult, error) {
	if !s.tryRegisterInboxSync(userID, accountID) {
		return SyncResult{}, ErrSyncAlreadyRunning
	}
	defer s.unregisterInboxSync(userID, accountID)

	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return SyncResult{}, err
	}
	return s.syncInbox(account, "manual")
}

func (s *Service) syncInbox(account storage.MailAccount, triggerType string) (SyncResult, error) {
	jobID, err := s.store.CreateSyncJob(account.UserID, account.ID, triggerType, "running")
	if err != nil {
		return SyncResult{}, err
	}

	config, err := s.accountConfig(account)
	if err != nil {
		_ = s.store.FinishSyncJob(jobID, "failed", 0, err.Error())
		_ = s.store.UpdateMailAccountSyncStatus(account.UserID, account.ID, "failed", err.Error())
		return SyncResult{JobID: jobID}, err
	}

	newCount := 0
	warnings := make([]string, 0)
	for _, folder := range accountSyncFolders(account) {
		folderConfig := configForFolder(config, folder)
		messages, err := s.fetcher.FetchFolder(folderConfig)
		if err != nil {
			if shouldSkipMissingOptionalFolder(folder, err) {
				warnings = append(warnings, fmt.Sprintf("文件夹 %s 不存在，已跳过", folder))
				continue
			}
			_ = s.store.FinishSyncJob(jobID, "failed", newCount, err.Error())
			_ = s.store.UpdateMailAccountSyncStatus(account.UserID, account.ID, "failed", err.Error())
			return SyncResult{JobID: jobID, NewMessageCount: newCount, Warnings: warnings}, err
		}
		for _, fetched := range messages {
			inserted, err := s.saveMessage(account.UserID, account.ID, folder, fetched)
			if err != nil {
				_ = s.store.FinishSyncJob(jobID, "failed", newCount, err.Error())
				_ = s.store.UpdateMailAccountSyncStatus(account.UserID, account.ID, "failed", err.Error())
				return SyncResult{JobID: jobID, NewMessageCount: newCount, Warnings: warnings}, err
			}
			if inserted {
				newCount++
			}
		}
	}

	_ = s.store.FinishSyncJob(jobID, "success", newCount, "")
	_ = s.store.UpdateMailAccountSyncStatus(account.UserID, account.ID, "success", "")
	return SyncResult{JobID: jobID, NewMessageCount: newCount, Warnings: warnings}, nil
}

func (s *Service) tryRegisterInboxSync(userID, accountID int64) bool {
	s.inboxSyncMu.Lock()
	defer s.inboxSyncMu.Unlock()
	key := fullSyncKey(userID, accountID)
	if _, ok := s.inboxSyncs[key]; ok {
		return false
	}
	s.inboxSyncs[key] = struct{}{}
	return true
}

func (s *Service) unregisterInboxSync(userID, accountID int64) {
	s.inboxSyncMu.Lock()
	defer s.inboxSyncMu.Unlock()
	delete(s.inboxSyncs, fullSyncKey(userID, accountID))
}

func (s *Service) StartFullSync(userID, accountID int64) (FullSyncStatus, error) {
	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return FullSyncStatus{}, err
	}
	if account.FullSyncStatus == "running" {
		return fullSyncStatusFromAccount(account), nil
	}
	if err := s.store.StartMailAccountFullSync(userID, accountID, 0); err != nil {
		return FullSyncStatus{}, err
	}
	cancel := s.registerFullSyncCancel(userID, accountID)

	go s.runFullSync(userID, accountID, cancel)

	account, err = s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return FullSyncStatus{}, err
	}
	return fullSyncStatusFromAccount(account), nil
}

func (s *Service) StopFullSync(userID, accountID int64) (FullSyncStatus, error) {
	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return FullSyncStatus{}, err
	}
	if account.FullSyncStatus != "running" {
		return fullSyncStatusFromAccount(account), nil
	}
	s.cancelFullSync(userID, accountID)
	if err := s.store.FinishMailAccountFullSync(userID, accountID, "cancelled", "用户停止了全量同步"); err != nil {
		return FullSyncStatus{}, err
	}
	account, err = s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return FullSyncStatus{}, err
	}
	return fullSyncStatusFromAccount(account), nil
}

func (s *Service) GetFullSyncStatus(userID, accountID int64) (FullSyncStatus, error) {
	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return FullSyncStatus{}, err
	}
	return fullSyncStatusFromAccount(account), nil
}

func (s *Service) runFullSync(userID, accountID int64, cancel <-chan struct{}) {
	defer s.unregisterFullSyncCancel(userID, accountID)
	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		_ = s.store.FinishMailAccountFullSync(userID, accountID, "failed", err.Error())
		return
	}
	config, err := s.accountConfig(account)
	if err != nil {
		_ = s.store.FinishMailAccountFullSync(userID, accountID, "failed", err.Error())
		_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "failed", err.Error())
		return
	}

	type folderUIDs struct {
		folder string
		uids   []string
	}
	folderBatches := make([]folderUIDs, 0)
	total := 0
	for _, folder := range accountSyncFolders(account) {
		folderConfig := configForFolder(config, folder)
		uids, err := s.fetcher.ListFolderUIDs(folderConfig)
		if err != nil {
			if shouldSkipMissingOptionalFolder(folder, err) {
				log.Printf("mail full sync skip missing optional folder account=%d user=%d folder=%s", accountID, userID, folder)
				continue
			}
			_ = s.store.FinishMailAccountFullSync(userID, accountID, "failed", err.Error())
			_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "failed", err.Error())
			return
		}
		reverseStrings(uids)
		folderBatches = append(folderBatches, folderUIDs{folder: folder, uids: uids})
		total += len(uids)
	}
	_ = s.store.UpdateMailAccountFullSyncProgress(userID, accountID, total, 0, 0)

	newCount := 0
	processed := 0
	for _, folderBatch := range folderBatches {
		folderConfig := configForFolder(config, folderBatch.folder)
		for start := 0; start < len(folderBatch.uids); start += fullSyncBatchSize {
			if s.fullSyncCancelled(cancel) {
				_ = s.store.FinishMailAccountFullSync(userID, accountID, "cancelled", "用户停止了全量同步")
				return
			}
			end := start + fullSyncBatchSize
			if end > len(folderBatch.uids) {
				end = len(folderBatch.uids)
			}
			batchUIDs := folderBatch.uids[start:end]
			messages, err := s.fetcher.FetchFolderByUIDs(folderConfig, batchUIDs)
			if err != nil {
				_ = s.store.FinishMailAccountFullSync(userID, accountID, "failed", err.Error())
				_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "failed", err.Error())
				return
			}
			if s.fullSyncCancelled(cancel) {
				_ = s.store.FinishMailAccountFullSync(userID, accountID, "cancelled", "用户停止了全量同步")
				return
			}
			for _, fetched := range messages {
				if s.fullSyncCancelled(cancel) {
					_ = s.store.FinishMailAccountFullSync(userID, accountID, "cancelled", "用户停止了全量同步")
					return
				}
				inserted, err := s.saveMessage(userID, accountID, folderBatch.folder, fetched)
				if err != nil {
					_ = s.store.FinishMailAccountFullSync(userID, accountID, "failed", err.Error())
					_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "failed", err.Error())
					return
				}
				if inserted {
					newCount++
				}
			}
			processed += len(batchUIDs)
			_ = s.store.UpdateMailAccountFullSyncProgress(userID, accountID, total, processed, newCount)
		}
	}
	if s.fullSyncCancelled(cancel) {
		_ = s.store.FinishMailAccountFullSync(userID, accountID, "cancelled", "用户停止了全量同步")
		return
	}

	if err := s.cleanupServerOldMessages(userID, accountID, account, config); err != nil {
		_ = s.store.FinishMailAccountFullSync(userID, accountID, "failed", err.Error())
		_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "failed", err.Error())
		return
	}

	_ = s.store.FinishMailAccountFullSync(userID, accountID, "success", "")
	_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "success", "")
}

func (s *Service) registerFullSyncCancel(userID, accountID int64) chan struct{} {
	s.fullSyncMu.Lock()
	defer s.fullSyncMu.Unlock()
	key := fullSyncKey(userID, accountID)
	if existing, ok := s.fullSyncCancels[key]; ok {
		close(existing)
	}
	cancel := make(chan struct{})
	s.fullSyncCancels[key] = cancel
	return cancel
}

func (s *Service) unregisterFullSyncCancel(userID, accountID int64) {
	s.fullSyncMu.Lock()
	defer s.fullSyncMu.Unlock()
	delete(s.fullSyncCancels, fullSyncKey(userID, accountID))
}

func (s *Service) cancelFullSync(userID, accountID int64) {
	s.fullSyncMu.Lock()
	defer s.fullSyncMu.Unlock()
	key := fullSyncKey(userID, accountID)
	if cancel, ok := s.fullSyncCancels[key]; ok {
		close(cancel)
		delete(s.fullSyncCancels, key)
	}
}

func (s *Service) fullSyncCancelled(cancel <-chan struct{}) bool {
	select {
	case <-cancel:
		return true
	default:
		return false
	}
}

func fullSyncKey(userID, accountID int64) string {
	return fmt.Sprintf("%d:%d", userID, accountID)
}

func (s *Service) cleanupServerOldMessages(userID, accountID int64, account storage.MailAccount, config AccountConfig) error {
	if !account.CleanupEnabled {
		return nil
	}
	retentionDays := account.CleanupRetentionDays
	if retentionDays <= 0 {
		retentionDays = 90
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	uids, err := s.store.ListSyncedInboxUIDsBefore(userID, accountID, cutoff)
	if err != nil {
		return err
	}
	if len(uids) == 0 {
		return nil
	}
	for start := 0; start < len(uids); start += fullSyncBatchSize {
		end := start + fullSyncBatchSize
		if end > len(uids) {
			end = len(uids)
		}
		if err := s.fetcher.DeleteInboxUIDs(config, uids[start:end]); err != nil {
			return err
		}
	}
	return nil
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

func (s *Service) smtpConfig(account storage.MailAccount) (SMTPConfig, error) {
	if strings.TrimSpace(account.SMTPHost) == "" || account.SMTPPort <= 0 {
		return SMTPConfig{}, fmt.Errorf("请先在邮箱账号中配置 SMTP 发信服务器")
	}
	password := ""
	if strings.TrimSpace(account.SMTPPasswordEncoded) != "" {
		decrypted, err := crypto.DecryptString(account.SMTPPasswordEncoded, s.credentialSecret)
		if err != nil {
			return SMTPConfig{}, err
		}
		password = decrypted
	} else if account.AuthType != "oauth2" && strings.TrimSpace(account.IMAPPasswordEncoded) != "" {
		decrypted, err := crypto.DecryptString(account.IMAPPasswordEncoded, s.credentialSecret)
		if err != nil {
			return SMTPConfig{}, err
		}
		password = decrypted
	}
	username := strings.TrimSpace(account.SMTPUsername)
	if username == "" {
		username = account.Email
	}
	return SMTPConfig{
		Email:       account.Email,
		DisplayName: account.DisplayName,
		Host:        account.SMTPHost,
		Port:        account.SMTPPort,
		TLS:         account.SMTPTLS,
		StartTLS:    account.SMTPStartTLS,
		Username:    username,
		Password:    password,
	}, nil
}

func accountSyncFolders(account storage.MailAccount) []string {
	folders := []string{"INBOX", normalizeSentFolder(account.SentFolder)}
	seen := make(map[string]bool, len(folders))
	unique := make([]string, 0, len(folders))
	for _, folder := range folders {
		folder = normalizeFolderName(folder)
		key := strings.ToLower(folder)
		if folder == "" || seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, folder)
	}
	return unique
}

func configForFolder(config AccountConfig, folder string) AccountConfig {
	config.Folder = normalizeFolderName(folder)
	return config
}

func shouldSkipMissingOptionalFolder(folder string, err error) bool {
	if !errors.Is(err, ErrFolderNotFound) {
		return false
	}
	return !strings.EqualFold(normalizeFolderName(folder), "INBOX")
}

func normalizeSentFolder(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Sent"
	}
	return value
}

func normalizeFolderName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "INBOX"
	}
	return value
}

func fullSyncStatusFromAccount(account storage.MailAccount) FullSyncStatus {
	status := strings.TrimSpace(account.FullSyncStatus)
	if status == "" {
		status = "idle"
	}
	return FullSyncStatus{
		Status:         status,
		Total:          account.FullSyncTotal,
		Processed:      account.FullSyncProcessed,
		NewCount:       account.FullSyncNewCount,
		StartedAt:      account.FullSyncStartedAt,
		FinishedAt:     account.FullSyncFinishedAt,
		Error:          account.FullSyncError,
		CleanupEnabled: account.CleanupEnabled,
		RetentionDays:  account.CleanupRetentionDays,
	}
}

func reverseStrings(values []string) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
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

func (s *Service) saveMessage(userID, accountID int64, folder string, fetched FetchedMessage) (bool, error) {
	folder = normalizeFolderName(folder)
	uid := strings.TrimSpace(fetched.UID)
	if uid == "" {
		uid = strings.TrimSpace(fetched.MessageID)
	}
	if uid == "" {
		uid = fmt.Sprintf("generated-%d", time.Now().UnixNano())
	}

	messageDir := filepath.Join(s.dataDir, "users", fmt.Sprint(userID), "accounts", fmt.Sprint(accountID), "messages", safePath(folder), safePath(uid))
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
		Folder:         folder,
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
	if err := s.upsertContactsFromFetchedMessage(userID, fetched, receivedAt); err != nil {
		log.Printf("upsert contacts from message user=%d account=%d uid=%s: %v", userID, accountID, uid, err)
	}
	if !inserted {
		if len(fetched.Attachments) > 0 {
			message, err := s.store.FindMailMessageByUID(userID, accountID, folder, uid)
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
	message, err := s.store.FindMailMessageByUID(userID, accountID, folder, uid)
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

func (s *Service) upsertContactsFromFetchedMessage(userID int64, fetched FetchedMessage, seenAt sql.NullTime) error {
	for _, candidate := range contactCandidatesFromFetchedMessage(fetched) {
		if _, err := s.store.UpsertContactSeen(storage.CreateContactParams{
			UserID:      userID,
			Email:       candidate.email,
			DisplayName: candidate.name,
			Source:      "auto",
			SeenAt:      seenAt,
		}); err != nil && !errors.Is(err, storage.ErrNotFound) {
			return err
		}
	}
	return nil
}

func (s *Service) upsertBCCContacts(userID int64, values []string, seenAt time.Time) error {
	for _, value := range values {
		for _, candidate := range parseContactCandidates(value) {
			if _, err := s.store.UpsertContactSeen(storage.CreateContactParams{
				UserID:      userID,
				Email:       candidate.email,
				DisplayName: candidate.name,
				Source:      "auto",
				SeenAt:      sql.NullTime{Time: seenAt, Valid: true},
			}); err != nil && !errors.Is(err, storage.ErrNotFound) {
				return err
			}
		}
	}
	return nil
}

type contactCandidate struct {
	email string
	name  string
}

func contactCandidatesFromFetchedMessage(fetched FetchedMessage) []contactCandidate {
	values := []string{fetched.From}
	values = append(values, fetched.To...)
	values = append(values, fetched.CC...)
	seen := make(map[string]bool)
	candidates := make([]contactCandidate, 0, len(values))
	for _, value := range values {
		for _, candidate := range parseContactCandidates(value) {
			key := strings.ToLower(candidate.email)
			if seen[key] {
				continue
			}
			seen[key] = true
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

func parseContactCandidates(value string) []contactCandidate {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	addresses, err := netmail.ParseAddressList(value)
	if err != nil {
		address, singleErr := netmail.ParseAddress(value)
		if singleErr != nil {
			return nil
		}
		addresses = []*netmail.Address{address}
	}
	candidates := make([]contactCandidate, 0, len(addresses))
	for _, address := range addresses {
		email := strings.ToLower(strings.TrimSpace(address.Address))
		if email == "" {
			continue
		}
		candidates = append(candidates, contactCandidate{
			email: email,
			name:  strings.TrimSpace(address.Name),
		})
	}
	return candidates
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

func valueOrExisting(value, existing string) string {
	if strings.TrimSpace(value) == "" {
		return existing
	}
	return value
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
