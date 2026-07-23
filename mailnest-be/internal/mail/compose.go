package mail

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"log"
	netmail "net/mail"
	"os"
	"strings"
	"time"

	"mailnest-be/internal/storage"
)

// ComposeAttachment 是回复/转发写信上下文中可带出的附件摘要。
type ComposeAttachment struct {
	ID          int64
	Filename    string
	ContentType string
	Size        int64
	Selected    bool
}

// ComposeContext 由后端统一生成，避免前端自行拼接回复/转发收件人和引用头。
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

const maxOutgoingAttachmentCount = 20

const maxOutgoingAttachmentBytes = 25 << 20

type SendMessageResult struct {
	Message storage.MailMessage
	Log     storage.MailSendLog
}

func (s *Service) SendMessage(userID int64, accountID int64, message OutgoingMessage) (storage.MailMessage, error) {
	result, err := s.SendMessageWithLog(userID, accountID, message)
	return result.Message, err
}

func (s *Service) SendMessageWithLog(userID int64, accountID int64, message OutgoingMessage) (SendMessageResult, error) {
	started := time.Now()
	log.Printf("发送邮件开始 userID=%d accountID=%d mode=%s attachments=%d forwardAttachments=%d", userID, accountID, normalizeComposeMode(message.ComposeMode), len(message.Attachments), len(message.ForwardAttachmentIDs))
	sendLog, err := s.store.CreateMailSendLog(storage.CreateMailSendLogParams{
		UserID:          userID,
		AccountID:       accountID,
		DraftID:         sql.NullInt64{Int64: message.DraftID, Valid: message.DraftID > 0},
		SourceMessageID: sql.NullInt64{Int64: message.SourceMessageID, Valid: message.SourceMessageID > 0},
		ComposeMode:     message.ComposeMode,
		RecipientsJSON:  recipientsSnapshot(message),
		Subject:         message.Subject,
		AttachmentCount: len(message.Attachments) + len(message.ForwardAttachmentIDs),
		StartedAt:       sql.NullTime{Time: started, Valid: true},
	})
	if err != nil {
		return SendMessageResult{}, err
	}
	fail := func(status string, retryStatus string, cause error) (SendMessageResult, error) {
		updated := s.updateSendLog(userID, sendLog.ID, storage.UpdateMailSendLogParams{
			Status:       status,
			RetryStatus:  retryStatus,
			ErrorMessage: sanitizeSendError(cause),
			FinishedAt:   sql.NullTime{Time: time.Now(), Valid: true},
		})
		if updated.ID != 0 {
			sendLog = updated
		}
		return SendMessageResult{Log: sendLog}, cause
	}
	account, err := s.store.FindMailAccountByID(userID, accountID)
	if err != nil {
		return fail("failed", "none", err)
	}
	if err := s.prepareOutgoingSource(userID, &message); err != nil {
		return fail("failed", "none", err)
	}
	if len(message.ForwardAttachmentIDs) > 0 {
		forwarded, err := s.forwardedAttachments(userID, message.SourceMessageID, message.ForwardAttachmentIDs)
		if err != nil {
			return fail("failed", "none", err)
		}
		message.Attachments = append(message.Attachments, forwarded...)
	}
	if err := validateOutgoingAttachments(message.Attachments); err != nil {
		return fail("failed", "none", err)
	}
	config, err := s.smtpConfig(account)
	if err != nil {
		return fail("failed", "none", err)
	}
	message.From = account.Email
	message.FromName = account.DisplayName
	result, err := s.sender.Send(config, message)
	if err != nil {
		log.Printf("发送邮件失败 userID=%d accountID=%d err=%v", userID, accountID, err)
		return fail("failed", "retryable", err)
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
		log.Printf("发送邮件已投递但本地保存失败 userID=%d accountID=%d hasMessageID=%t err=%v", userID, accountID, strings.TrimSpace(result.MessageID) != "", err)
		return fail("local_save_failed", "none", err)
	}
	if err := s.upsertBCCContacts(userID, message.BCC, result.SentAt); err != nil {
		log.Printf("发送邮件后沉淀密送联系人失败 userID=%d accountID=%d err=%v", userID, accountID, err)
	}
	log.Printf("发送邮件完成 userID=%d accountID=%d hasMessageID=%t duration=%s", userID, accountID, strings.TrimSpace(result.MessageID) != "", time.Since(started))
	sent, err := s.store.FindMailMessageByUID(userID, accountID, normalizeSentFolder(account.SentFolder), fetched.UID)
	if err != nil {
		return fail("local_save_failed", "none", err)
	}
	sendLog = s.updateSendLog(userID, sendLog.ID, storage.UpdateMailSendLogParams{
		MessageID:     sql.NullInt64{Int64: sent.ID, Valid: true},
		SMTPMessageID: result.MessageID,
		Status:        "success",
		RetryStatus:   "none",
		FinishedAt:    sql.NullTime{Time: time.Now(), Valid: true},
	})
	return SendMessageResult{Message: sent, Log: sendLog}, nil
}

func (s *Service) updateSendLog(userID, id int64, params storage.UpdateMailSendLogParams) storage.MailSendLog {
	params.UserID = userID
	params.ID = id
	logItem, err := s.store.UpdateMailSendLog(params)
	if err != nil {
		log.Printf("更新发信记录失败 userID=%d sendLogID=%d err=%v", userID, id, err)
		return storage.MailSendLog{}
	}
	return logItem
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

func recipientsSnapshot(message OutgoingMessage) string {
	payload := map[string][]string{
		"to":  nonEmptyStrings(message.To),
		"cc":  nonEmptyStrings(message.CC),
		"bcc": nonEmptyStrings(message.BCC),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return `{"to":[],"cc":[],"bcc":[]}`
	}
	return string(data)
}

func sanitizeSendError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len([]rune(message)) > 500 {
		message = string([]rune(message)[:500])
	}
	return message
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
