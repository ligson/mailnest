package mail

import (
	"context"
	"database/sql"
	"encoding/json"
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

type ComposeAttachment struct {
	ID          int64
	Filename    string
	ContentType string
	Size        int64
	Selected    bool
}

type ComposeContext struct {
	Mode               string
	SourceMessageID    int64
	AccountID          int64
	To                 []string
	CC                 []string
	BCC                []string
	Subject            string
	TextBody           string
	HTMLBody           string
	ForwardAttachments []ComposeAttachment
}

const fullSyncBatchSize = 50
const parsedContentRepairLimit = 5000
const defaultAutoSyncCheckInterval = time.Minute
const defaultAutoSyncBatchLimit = 20
const defaultAutoSyncMaxConcurrent = 2
const maxOutgoingAttachmentCount = 20
const maxOutgoingAttachmentBytes = 25 << 20

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
	if err := s.prepareOutgoingSource(userID, &message); err != nil {
		return storage.MailMessage{}, err
	}
	if len(message.ForwardAttachmentIDs) > 0 {
		forwarded, err := s.forwardedAttachments(userID, message.SourceMessageID, message.ForwardAttachmentIDs)
		if err != nil {
			return storage.MailMessage{}, err
		}
		message.Attachments = append(message.Attachments, forwarded...)
	}
	if err := validateOutgoingAttachments(message.Attachments); err != nil {
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
		UID:             "sent-" + safePath(strings.Trim(result.MessageID, "<>")),
		MessageID:       result.MessageID,
		InReplyTo:       message.InReplyTo,
		References:      strings.Join(normalizedReferences(message.References), " "),
		SourceMessageID: message.SourceMessageID,
		ComposeMode:     normalizeComposeMode(message.ComposeMode),
		Subject:         strings.TrimSpace(message.Subject),
		From:            (&netmail.Address{Name: account.DisplayName, Address: account.Email}).String(),
		To:              nonEmptyStrings(message.To),
		CC:              nonEmptyStrings(message.CC),
		SentAt:          result.SentAt.Format(time.RFC3339),
		TextBody:        message.TextBody,
		HTMLBody:        message.HTMLBody,
		RawContent:      result.Raw,
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

func (s *Service) GetComposeContext(userID, messageID int64, mode string) (ComposeContext, error) {
	mode = normalizeComposeMode(mode)
	if mode == "new" {
		mode = "reply"
	}
	source, err := s.store.FindMailMessageByID(userID, messageID)
	if err != nil {
		return ComposeContext{}, err
	}

	textBody := readContentFile(nullableStringValue(source.TextBodyPath))
	htmlBody := stripUnsafeQuoteHTML(readContentFile(nullableStringValue(source.HTMLBodyPath)))
	if strings.TrimSpace(htmlBody) == "" && strings.TrimSpace(textBody) != "" {
		htmlBody = "<pre>" + html.EscapeString(textBody) + "</pre>"
	}
	ctx := ComposeContext{
		Mode:            mode,
		SourceMessageID: source.ID,
		AccountID:       source.AccountID,
		Subject:         composeSubject(nullableStringValue(source.Subject), mode),
		BCC:             []string{},
	}

	switch mode {
	case "forward":
		ctx.TextBody = s.forwardTextBody(userID, source, textBody)
		ctx.HTMLBody = s.forwardHTMLBody(userID, source, htmlBody)
		attachments, err := s.store.ListMailAttachments(userID, source.ID)
		if err != nil {
			return ComposeContext{}, err
		}
		for _, attachment := range attachments {
			if attachment.Inline {
				continue
			}
			ctx.ForwardAttachments = append(ctx.ForwardAttachments, ComposeAttachment{
				ID:          attachment.ID,
				Filename:    attachment.Filename,
				ContentType: nullableStringValue(attachment.ContentType),
				Size:        attachment.Size,
				Selected:    true,
			})
		}
	case "replyAll":
		ctx.To, ctx.CC = s.replyAllRecipients(userID, source)
		ctx.TextBody = s.replyTextBody(userID, source, textBody)
		ctx.HTMLBody = s.replyHTMLBody(userID, source, htmlBody)
	default:
		ctx.Mode = "reply"
		ctx.To = s.firstAddressFromField(userID, source.FromAddr)
		ctx.CC = []string{}
		ctx.TextBody = s.replyTextBody(userID, source, textBody)
		ctx.HTMLBody = s.replyHTMLBody(userID, source, htmlBody)
	}
	return ctx, nil
}

func (s *Service) prepareOutgoingSource(userID int64, message *OutgoingMessage) error {
	mode := normalizeComposeMode(message.ComposeMode)
	message.ComposeMode = mode
	if mode == "new" {
		message.SourceMessageID = 0
		message.InReplyTo = ""
		message.References = nil
		return nil
	}
	if message.SourceMessageID <= 0 {
		return fmt.Errorf("回复或转发需要来源邮件")
	}
	source, err := s.store.FindMailMessageByID(userID, message.SourceMessageID)
	if err != nil {
		return err
	}
	if mode == "reply" || mode == "replyAll" {
		message.InReplyTo = nullableStringValue(source.MessageID)
		message.References = referencesForSource(source)
		return nil
	}
	message.InReplyTo = ""
	message.References = nil
	return nil
}

func (s *Service) forwardedAttachments(userID, sourceMessageID int64, ids []int64) ([]OutgoingAttachment, error) {
	if sourceMessageID <= 0 {
		return nil, fmt.Errorf("转发附件需要来源邮件")
	}
	seen := make(map[int64]bool)
	attachments := make([]OutgoingAttachment, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		attachment, err := s.store.FindMailAttachmentByID(userID, sourceMessageID, id)
		if err != nil {
			return nil, err
		}
		if attachment.Inline {
			continue
		}
		data, err := os.ReadFile(attachment.FilePath)
		if err != nil {
			return nil, fmt.Errorf("读取转发附件失败：%s", attachment.Filename)
		}
		attachments = append(attachments, OutgoingAttachment{
			Filename:    attachment.Filename,
			ContentType: nullableStringValue(attachment.ContentType),
			Data:        data,
		})
	}
	return attachments, nil
}

func validateOutgoingAttachments(attachments []OutgoingAttachment) error {
	if len(attachments) > maxOutgoingAttachmentCount {
		return fmt.Errorf("附件不能超过 %d 个", maxOutgoingAttachmentCount)
	}
	var total int64
	for _, attachment := range attachments {
		total += int64(len(attachment.Data))
		if total > maxOutgoingAttachmentBytes {
			return fmt.Errorf("附件总大小不能超过 %d MB", maxOutgoingAttachmentBytes>>20)
		}
	}
	return nil
}

func (s *Service) replyAllRecipients(userID int64, source storage.MailMessage) ([]string, []string) {
	own := s.ownEmailKeys(userID)
	to := make([]string, 0)
	cc := make([]string, 0)
	seen := make(map[string]bool)
	for _, address := range addressesFromField(source.FromAddr) {
		if s.addRecipient(userID, &to, seen, own, address) {
			break
		}
	}
	for _, address := range append(addressesFromField(source.ToAddrs), addressesFromField(source.CCAddrs)...) {
		s.addRecipient(userID, &cc, seen, own, address)
	}
	if len(to) == 0 && len(cc) > 0 {
		to = append(to, cc[0])
		cc = cc[1:]
	}
	return to, cc
}

func (s *Service) ownEmailKeys(userID int64) map[string]bool {
	accounts, err := s.store.ListMailAccounts(userID)
	if err != nil {
		return map[string]bool{}
	}
	keys := make(map[string]bool, len(accounts)*2)
	for _, account := range accounts {
		addEmailKey(keys, account.Email)
		addEmailKey(keys, account.IMAPUsername)
	}
	return keys
}

func (s *Service) addRecipient(userID int64, out *[]string, seen map[string]bool, own map[string]bool, value string) bool {
	address, err := netmail.ParseAddress(value)
	if err != nil || strings.TrimSpace(address.Address) == "" {
		return false
	}
	key := strings.ToLower(strings.TrimSpace(address.Address))
	if own[key] || seen[key] {
		return false
	}
	seen[key] = true
	*out = append(*out, s.displayAddressForUser(userID, address))
	return true
}

func addEmailKey(keys map[string]bool, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if address, err := netmail.ParseAddress(value); err == nil && strings.TrimSpace(address.Address) != "" {
		keys[strings.ToLower(strings.TrimSpace(address.Address))] = true
		return
	}
	if strings.Contains(value, "@") {
		keys[strings.ToLower(value)] = true
	}
}

func addressesFromField(value sql.NullString) []string {
	raw := nullableStringValue(value)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	addresses, err := netmail.ParseAddressList(raw)
	if err != nil {
		parts := strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == ';' || r == '，' || r == '；'
		})
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if address, err := netmail.ParseAddress(strings.TrimSpace(part)); err == nil {
				out = append(out, displayAddress(address))
			}
		}
		return out
	}
	out := make([]string, 0, len(addresses))
	for _, address := range addresses {
		out = append(out, displayAddress(address))
	}
	return out
}

func (s *Service) firstAddressFromField(userID int64, value sql.NullString) []string {
	addresses := addressesFromField(value)
	if len(addresses) == 0 {
		return nil
	}
	address, err := netmail.ParseAddress(addresses[0])
	if err != nil {
		return addresses[:1]
	}
	return []string{s.displayAddressForUser(userID, address)}
}

func displayAddress(address *netmail.Address) string {
	if address == nil || strings.TrimSpace(address.Address) == "" {
		return ""
	}
	name := strings.TrimSpace(address.Name)
	email := strings.TrimSpace(address.Address)
	if name == "" {
		return email
	}
	return name + " <" + email + ">"
}

func (s *Service) displayAddressForUser(userID int64, address *netmail.Address) string {
	if address == nil || strings.TrimSpace(address.Address) == "" {
		return ""
	}
	name := strings.TrimSpace(address.Name)
	if contact, err := s.store.FindContactByEmail(userID, address.Address); err == nil {
		if preferredName := contactPreferredName(contact); preferredName != "" {
			name = preferredName
		}
	}
	return displayAddress(&netmail.Address{Name: name, Address: strings.TrimSpace(address.Address)})
}

func contactPreferredName(contact storage.Contact) string {
	if contact.Nickname.Valid && strings.TrimSpace(contact.Nickname.String) != "" {
		return strings.TrimSpace(contact.Nickname.String)
	}
	if contact.DisplayName.Valid && strings.TrimSpace(contact.DisplayName.String) != "" {
		return strings.TrimSpace(contact.DisplayName.String)
	}
	return ""
}

func (s *Service) displayAddressField(userID int64, value sql.NullString) string {
	values := addressesFromField(value)
	for i, value := range values {
		address, err := netmail.ParseAddress(value)
		if err != nil {
			continue
		}
		values[i] = s.displayAddressForUser(userID, address)
	}
	return strings.Join(values, ", ")
}

func referencesForSource(source storage.MailMessage) []string {
	refs := make([]string, 0)
	if source.References.Valid {
		refs = append(refs, strings.Fields(source.References.String)...)
	}
	if source.MessageID.Valid && strings.TrimSpace(source.MessageID.String) != "" {
		refs = append(refs, strings.TrimSpace(source.MessageID.String))
	}
	return normalizedReferences(refs)
}

func normalizeComposeMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "reply":
		return "reply"
	case "replyall", "reply_all", "reply-all":
		return "replyAll"
	case "forward", "fwd":
		return "forward"
	default:
		return "new"
	}
}

func composeSubject(subject, mode string) string {
	subject = strings.TrimSpace(subject)
	switch normalizeComposeMode(mode) {
	case "forward":
		if hasSubjectPrefix(subject, "fwd:") || hasSubjectPrefix(subject, "fw:") || hasSubjectPrefix(subject, "转发:") {
			return subject
		}
		return "Fwd: " + subject
	default:
		if hasSubjectPrefix(subject, "re:") || hasSubjectPrefix(subject, "回复:") {
			return subject
		}
		return "Re: " + subject
	}
}

func hasSubjectPrefix(subject, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(subject)), strings.ToLower(prefix))
}

func (s *Service) replyTextBody(userID int64, source storage.MailMessage, body string) string {
	return "\n\n在 " + messageQuoteTime(source) + "，" + s.quoteAuthor(userID, source) + " 写道：\n" + quoteText(body)
}

func (s *Service) forwardTextBody(userID int64, source storage.MailMessage, body string) string {
	return "\n\n---------- 转发邮件 ----------\n" +
		"发件人：" + s.displayAddressField(userID, source.FromAddr) + "\n" +
		"收件人：" + s.displayAddressField(userID, source.ToAddrs) + "\n" +
		"抄送：" + s.displayAddressField(userID, source.CCAddrs) + "\n" +
		"日期：" + messageQuoteTime(source) + "\n" +
		"主题：" + nullableStringValue(source.Subject) + "\n\n" +
		body
}

func (s *Service) replyHTMLBody(userID int64, source storage.MailMessage, body string) string {
	if strings.TrimSpace(body) == "" {
		body = "<p>没有正文内容</p>"
	}
	return `<p><br></p><blockquote style="border-left:3px solid #d9d9d9;margin:12px 0;padding:0 0 0 12px;color:#5f6b7a;">` +
		`<p>在 ` + html.EscapeString(messageQuoteTime(source)) + `，` + html.EscapeString(s.quoteAuthor(userID, source)) + ` 写道：</p>` +
		body + `</blockquote>`
}

func (s *Service) forwardHTMLBody(userID int64, source storage.MailMessage, body string) string {
	if strings.TrimSpace(body) == "" {
		body = "<p>没有正文内容</p>"
	}
	header := `<p><br></p><div style="border-top:1px solid #d9d9d9;margin-top:16px;padding-top:12px;">` +
		`<p><strong>转发邮件</strong></p>` +
		`<p>发件人：` + html.EscapeString(s.displayAddressField(userID, source.FromAddr)) + `<br>` +
		`收件人：` + html.EscapeString(s.displayAddressField(userID, source.ToAddrs)) + `<br>` +
		`抄送：` + html.EscapeString(s.displayAddressField(userID, source.CCAddrs)) + `<br>` +
		`日期：` + html.EscapeString(messageQuoteTime(source)) + `<br>` +
		`主题：` + html.EscapeString(nullableStringValue(source.Subject)) + `</p></div>`
	return header + `<blockquote style="border-left:3px solid #d9d9d9;margin:12px 0;padding:0 0 0 12px;color:#5f6b7a;">` + body + `</blockquote>`
}

func quoteText(value string) string {
	value = strings.TrimRight(value, "\r\n")
	if value == "" {
		return "> 没有正文内容"
	}
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for i, line := range lines {
		lines[i] = "> " + line
	}
	return strings.Join(lines, "\n")
}

func (s *Service) quoteAuthor(userID int64, source storage.MailMessage) string {
	from := nullableStringValue(source.FromAddr)
	if address, err := netmail.ParseAddress(from); err == nil {
		if contactName := contactNameForAddress(s.store, userID, address.Address); contactName != "" {
			return contactName
		}
		if strings.TrimSpace(address.Name) != "" {
			return strings.TrimSpace(address.Name)
		}
		return strings.TrimSpace(address.Address)
	}
	return from
}

func contactNameForAddress(store *storage.Store, userID int64, email string) string {
	contact, err := store.FindContactByEmail(userID, email)
	if err != nil {
		return ""
	}
	return contactPreferredName(contact)
}

func messageQuoteTime(source storage.MailMessage) string {
	for _, value := range []sql.NullTime{source.SentAt, source.ReceivedAt} {
		if value.Valid {
			return value.Time.Format("2006-01-02 15:04")
		}
	}
	return ""
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
			InReplyTo:    parsed.InReplyTo,
			References:   parsed.References,
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
	_ = s.store.CreateSyncJobEvent(jobID, "info", "start", "开始收取邮件", mustJSON(map[string]any{
		"triggerType": triggerType,
		"accountId":   account.ID,
	}))

	config, err := s.accountConfig(account)
	if err != nil {
		_ = s.store.FinishSyncJob(jobID, "failed", 0, err.Error())
		_ = s.store.CreateSyncJobEvent(jobID, "error", "config", err.Error(), mustJSON(nil))
		_ = s.store.UpdateMailAccountSyncStatus(account.UserID, account.ID, "failed", err.Error())
		return SyncResult{JobID: jobID}, err
	}

	newCount := 0
	warnings := make([]string, 0)
	for _, folder := range accountSyncFolders(account) {
		_ = s.store.CreateSyncJobEvent(jobID, "info", "folder", "正在同步文件夹 "+folder, mustJSON(map[string]any{
			"folder": folder,
		}))
		folderConfig := configForFolder(config, folder)
		messages, err := s.fetcher.FetchFolder(folderConfig)
		if err != nil {
			if shouldSkipMissingOptionalFolder(folder, err) {
				warnings = append(warnings, fmt.Sprintf("文件夹 %s 不存在，已跳过", folder))
				_ = s.store.CreateSyncJobEvent(jobID, "warn", "folder", "文件夹 "+folder+" 不存在，已跳过", mustJSON(map[string]any{
					"folder": folder,
				}))
				continue
			}
			_ = s.store.FinishSyncJob(jobID, "failed", newCount, err.Error())
			_ = s.store.CreateSyncJobEvent(jobID, "error", "folder", err.Error(), mustJSON(map[string]any{
				"folder": folder,
			}))
			_ = s.store.UpdateMailAccountSyncStatus(account.UserID, account.ID, "failed", err.Error())
			return SyncResult{JobID: jobID, NewMessageCount: newCount, Warnings: warnings}, err
		}
		for _, fetched := range messages {
			inserted, err := s.saveMessage(account.UserID, account.ID, folder, fetched)
			if err != nil {
				_ = s.store.FinishSyncJob(jobID, "failed", newCount, err.Error())
				_ = s.store.CreateSyncJobEvent(jobID, "error", "message", err.Error(), mustJSON(map[string]any{
					"folder": folder,
					"uid":    fetched.UID,
				}))
				_ = s.store.UpdateMailAccountSyncStatus(account.UserID, account.ID, "failed", err.Error())
				return SyncResult{JobID: jobID, NewMessageCount: newCount, Warnings: warnings}, err
			}
			if inserted {
				newCount++
			}
		}
		_ = s.store.CreateSyncJobEvent(jobID, "info", "folder", "文件夹 "+folder+" 同步完成", mustJSON(map[string]any{
			"folder": folder,
		}))
	}

	_ = s.store.FinishSyncJob(jobID, "success", newCount, "")
	_ = s.store.CreateSyncJobEvent(jobID, "info", "finish", "收取完成", mustJSON(map[string]any{
		"newMessageCount": newCount,
	}))
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
	jobID, err := s.store.CreateSyncJob(userID, accountID, "full", "running")
	if err != nil {
		log.Printf("create full sync job failed user=%d account=%d err=%v", userID, accountID, err)
		jobID = 0
	} else {
		_ = s.store.CreateSyncJobEvent(jobID, "info", "start", "开始全量同步", mustJSON(map[string]any{
			"userId":    userID,
			"accountId": accountID,
		}))
	}
	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		if jobID > 0 {
			_ = s.store.FinishSyncJob(jobID, "failed", 0, err.Error())
			_ = s.store.CreateSyncJobEvent(jobID, "error", "account", err.Error(), mustJSON(nil))
		}
		_ = s.store.FinishMailAccountFullSync(userID, accountID, "failed", err.Error())
		return
	}
	config, err := s.accountConfig(account)
	if err != nil {
		if jobID > 0 {
			_ = s.store.FinishSyncJob(jobID, "failed", 0, err.Error())
			_ = s.store.CreateSyncJobEvent(jobID, "error", "config", err.Error(), mustJSON(nil))
		}
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
				if jobID > 0 {
					_ = s.store.CreateSyncJobEvent(jobID, "warn", "folder", "文件夹 "+folder+" 不存在，已跳过", mustJSON(map[string]any{
						"folder": folder,
					}))
				}
				continue
			}
			if jobID > 0 {
				_ = s.store.FinishSyncJob(jobID, "failed", 0, err.Error())
				_ = s.store.CreateSyncJobEvent(jobID, "error", "folder", err.Error(), mustJSON(map[string]any{
					"folder": folder,
				}))
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
				if jobID > 0 {
					_ = s.store.FinishSyncJob(jobID, "cancelled", newCount, "用户停止了全量同步")
					_ = s.store.CreateSyncJobEvent(jobID, "warn", "cancel", "用户停止了全量同步", mustJSON(nil))
				}
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
				if jobID > 0 {
					_ = s.store.FinishSyncJob(jobID, "failed", processed, err.Error())
					_ = s.store.CreateSyncJobEvent(jobID, "error", "batch", err.Error(), mustJSON(map[string]any{
						"folder": folderBatch.folder,
					}))
				}
				_ = s.store.FinishMailAccountFullSync(userID, accountID, "failed", err.Error())
				_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "failed", err.Error())
				return
			}
			if s.fullSyncCancelled(cancel) {
				if jobID > 0 {
					_ = s.store.FinishSyncJob(jobID, "cancelled", newCount, "用户停止了全量同步")
					_ = s.store.CreateSyncJobEvent(jobID, "warn", "cancel", "用户停止了全量同步", mustJSON(nil))
				}
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
					if jobID > 0 {
						_ = s.store.FinishSyncJob(jobID, "failed", processed, err.Error())
						_ = s.store.CreateSyncJobEvent(jobID, "error", "message", err.Error(), mustJSON(map[string]any{
							"folder": folderBatch.folder,
							"uid":    fetched.UID,
						}))
					}
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
			if jobID > 0 {
				_ = s.store.CreateSyncJobEvent(jobID, "info", "batch", "批量同步完成", mustJSON(map[string]any{
					"folder":    folderBatch.folder,
					"processed": processed,
					"total":     total,
				}))
			}
		}
	}
	if s.fullSyncCancelled(cancel) {
		if jobID > 0 {
			_ = s.store.FinishSyncJob(jobID, "cancelled", newCount, "用户停止了全量同步")
			_ = s.store.CreateSyncJobEvent(jobID, "warn", "cancel", "用户停止了全量同步", mustJSON(nil))
		}
		_ = s.store.FinishMailAccountFullSync(userID, accountID, "cancelled", "用户停止了全量同步")
		return
	}

	if err := s.cleanupServerOldMessages(userID, accountID, account, config); err != nil {
		if jobID > 0 {
			_ = s.store.FinishSyncJob(jobID, "failed", newCount, err.Error())
			_ = s.store.CreateSyncJobEvent(jobID, "warn", "cleanup", err.Error(), mustJSON(nil))
		}
		_ = s.store.FinishMailAccountFullSync(userID, accountID, "failed", err.Error())
		_ = s.store.UpdateMailAccountSyncStatus(userID, accountID, "failed", err.Error())
		return
	}

	if jobID > 0 {
		_ = s.store.FinishSyncJob(jobID, "success", newCount, "")
		_ = s.store.CreateSyncJobEvent(jobID, "info", "finish", "全量同步完成", mustJSON(map[string]any{
			"newMessageCount": newCount,
		}))
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
		UserID:          userID,
		AccountID:       accountID,
		Folder:          folder,
		IMAPUID:         uid,
		MessageID:       fetched.MessageID,
		Subject:         fetched.Subject,
		FromAddr:        fetched.From,
		ToAddrs:         toAddrs,
		CCAddrs:         ccAddrs,
		SentAt:          sentAt,
		ReceivedAt:      receivedAt,
		HasAttachments:  len(fetched.Attachments) > 0,
		TextBodyPath:    textPath,
		HTMLBodyPath:    htmlPath,
		RawPath:         rawPath,
		SearchText:      buildSearchText(fetched, toAddrs, ccAddrs),
		InReplyTo:       fetched.InReplyTo,
		References:      fetched.References,
		SourceMessageID: sql.NullInt64{Int64: fetched.SourceMessageID, Valid: fetched.SourceMessageID > 0},
		ComposeMode:     fetched.ComposeMode,
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
var htmlScriptPattern = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)
var htmlEventAttrPattern = regexp.MustCompile(`(?is)\s+on[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
var htmlJavascriptURLPattern = regexp.MustCompile(`(?is)(href|src)\s*=\s*("|')\s*javascript:[^"']*("|')`)
var htmlImageTagPattern = regexp.MustCompile(`(?is)<img\b[^>]*>`)

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

func stripUnsafeQuoteHTML(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	value = htmlScriptPattern.ReplaceAllString(value, "")
	value = htmlEventAttrPattern.ReplaceAllString(value, "")
	value = htmlJavascriptURLPattern.ReplaceAllString(value, `$1="#"`)
	value = htmlImageTagPattern.ReplaceAllString(value, `<span style="color:#8c8c8c;">[内嵌图片已省略]</span>`)
	return value
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

func mustJSON(v any) string {
	if v == nil {
		return ""
	}
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}
