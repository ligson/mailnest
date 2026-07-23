package mail

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	stdmime "mime"
	"mime/quotedprintable"
	"net"
	stdmail "net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-sasl"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

type IMAPFetcher struct {
	Limit uint32
}

func NewIMAPFetcher() *IMAPFetcher {
	return &IMAPFetcher{Limit: 50}
}

func (f *IMAPFetcher) TestConnection(account AccountConfig) error {
	c, err := f.dial(account)
	if err != nil {
		return err
	}
	defer c.Logout()

	if err := authenticate(c, account); err != nil {
		return err
	}
	_, err = c.Select(folderName(account), true)
	return wrapFolderError(folderName(account), err)
}

func (f *IMAPFetcher) ListFolders(account AccountConfig) ([]FolderInfo, error) {
	c, err := f.dial(account)
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	if err := authenticate(c, account); err != nil {
		return nil, err
	}

	mailboxes := make(chan *imap.MailboxInfo, 64)
	done := make(chan error, 1)
	go func() {
		done <- c.List("", "*", mailboxes)
	}()

	folders := make([]FolderInfo, 0)
	for mailbox := range mailboxes {
		if mailbox == nil {
			continue
		}
		folders = append(folders, FolderInfo{
			Name:       mailbox.Name,
			Delimiter:  mailbox.Delimiter,
			Attributes: append([]string{}, mailbox.Attributes...),
		})
	}
	if err := <-done; err != nil {
		return nil, err
	}
	return folders, nil
}

func (f *IMAPFetcher) FetchInbox(account AccountConfig) ([]FetchedMessage, error) {
	return f.FetchFolder(account)
}

func (f *IMAPFetcher) FetchFolder(account AccountConfig) ([]FetchedMessage, error) {
	c, err := f.dial(account)
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	if err := authenticate(c, account); err != nil {
		return nil, err
	}

	folder := folderName(account)
	mbox, err := c.Select(folder, false)
	if err != nil {
		return nil, wrapFolderError(folder, err)
	}
	if mbox.Messages == 0 {
		return []FetchedMessage{}, nil
	}

	from := uint32(1)
	if mbox.Messages > f.Limit {
		from = mbox.Messages - f.Limit + 1
	}
	seqSet := new(imap.SeqSet)
	seqSet.AddRange(from, mbox.Messages)

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchUid, section.FetchItem()}
	messages := make(chan *imap.Message, int(mbox.Messages-from+1))
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqSet, items, messages)
	}()

	fetched := make([]FetchedMessage, 0)
	for msg := range messages {
		if msg == nil {
			continue
		}
		fetched = append(fetched, fetchedMessageFromIMAP(msg, section))
	}
	if err := <-done; err != nil {
		return nil, err
	}

	return fetched, nil
}

func (f *IMAPFetcher) ListInboxUIDs(account AccountConfig) ([]string, error) {
	return f.ListFolderUIDs(account)
}

func (f *IMAPFetcher) ListFolderUIDs(account AccountConfig) ([]string, error) {
	c, err := f.dial(account)
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	if err := authenticate(c, account); err != nil {
		return nil, err
	}
	folder := folderName(account)
	if _, err := c.Select(folder, true); err != nil {
		return nil, wrapFolderError(folder, err)
	}
	rawUIDs, err := c.UidSearch(&imap.SearchCriteria{})
	if err != nil {
		return nil, err
	}
	uids := make([]string, 0, len(rawUIDs))
	for _, uid := range rawUIDs {
		uids = append(uids, fmt.Sprint(uid))
	}
	return uids, nil
}

func (f *IMAPFetcher) FetchInboxByUIDs(account AccountConfig, uids []string) ([]FetchedMessage, error) {
	return f.FetchFolderByUIDs(account, uids)
}

func (f *IMAPFetcher) FetchFolderByUIDs(account AccountConfig, uids []string) ([]FetchedMessage, error) {
	if len(uids) == 0 {
		return []FetchedMessage{}, nil
	}
	c, err := f.dial(account)
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	if err := authenticate(c, account); err != nil {
		return nil, err
	}
	folder := folderName(account)
	if _, err := c.Select(folder, false); err != nil {
		return nil, wrapFolderError(folder, err)
	}
	seqSet, err := uidSeqSet(uids)
	if err != nil {
		return nil, err
	}
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchUid, section.FetchItem()}
	messages := make(chan *imap.Message, len(uids))
	done := make(chan error, 1)
	go func() {
		done <- c.UidFetch(seqSet, items, messages)
	}()

	fetched := make([]FetchedMessage, 0, len(uids))
	for msg := range messages {
		if msg == nil {
			continue
		}
		fetched = append(fetched, fetchedMessageFromIMAP(msg, section))
	}
	if err := <-done; err != nil {
		return nil, err
	}
	return fetched, nil
}

func (f *IMAPFetcher) DeleteInboxUIDs(account AccountConfig, uids []string) error {
	return f.DeleteFolderUIDs(account, uids)
}

func (f *IMAPFetcher) DeleteFolderUIDs(account AccountConfig, uids []string) error {
	if len(uids) == 0 {
		return nil
	}
	c, err := f.dial(account)
	if err != nil {
		return err
	}
	defer c.Logout()

	if err := authenticate(c, account); err != nil {
		return err
	}
	folder := folderName(account)
	if _, err := c.Select(folder, false); err != nil {
		return wrapFolderError(folder, err)
	}
	seqSet, err := uidSeqSet(uids)
	if err != nil {
		return err
	}
	if err := c.UidStore(seqSet, imap.AddFlags, []interface{}{imap.DeletedFlag}, nil); err != nil {
		return err
	}
	return c.Expunge(nil)
}

func fetchedMessageFromIMAP(msg *imap.Message, section *imap.BodySectionName) FetchedMessage {
	item := FetchedMessage{
		UID: fmt.Sprint(msg.Uid),
	}
	if msg.Envelope != nil {
		item.MessageID = msg.Envelope.MessageId
		item.InReplyTo = strings.TrimSpace(msg.Envelope.InReplyTo)
		item.Subject = decodeMIMEHeader(msg.Envelope.Subject)
		item.From = addressList(msg.Envelope.From)
		item.To = addressSlice(msg.Envelope.To)
		item.CC = addressSlice(msg.Envelope.Cc)
		if !msg.Envelope.Date.IsZero() {
			item.SentAt = msg.Envelope.Date.Format(time.RFC3339)
		}
	}
	if literal := msg.GetBody(section); literal != nil {
		raw, _ := io.ReadAll(literal)
		item.RawContent = string(raw)
		textBody, htmlBody, attachments := parseMessageBodies(raw)
		item.TextBody = textBody
		item.HTMLBody = htmlBody
		item.Attachments = attachments
	}
	return item
}

func fetchedMessageFromRaw(raw []byte) FetchedMessage {
	raw = normalizeMIMEMessage(raw)
	item := FetchedMessage{RawContent: string(raw)}
	if message, err := stdmail.ReadMessage(bytes.NewReader(raw)); err == nil {
		item.MessageID = strings.TrimSpace(message.Header.Get("Message-Id"))
		item.InReplyTo = strings.TrimSpace(message.Header.Get("In-Reply-To"))
		item.References = strings.TrimSpace(message.Header.Get("References"))
		item.Subject = decodeMIMEHeader(message.Header.Get("Subject"))
		item.From = parseAddressHeader(message.Header.Get("From"))
		item.To = parseAddressListHeader(message.Header.Get("To"))
		item.CC = parseAddressListHeader(message.Header.Get("Cc"))
		if dateValue := strings.TrimSpace(message.Header.Get("Date")); dateValue != "" {
			item.SentAt = dateValue
		}
	}
	textBody, htmlBody, attachments := parseMessageBodies(raw)
	item.TextBody = textBody
	item.HTMLBody = htmlBody
	item.Attachments = attachments
	return item
}

func uidSeqSet(uids []string) (*imap.SeqSet, error) {
	seqSet := new(imap.SeqSet)
	for _, uid := range uids {
		parsed, err := strconv.ParseUint(strings.TrimSpace(uid), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("IMAP UID 格式错误：%s", uid)
		}
		seqSet.AddNum(uint32(parsed))
	}
	return seqSet, nil
}

func authenticate(c *client.Client, account AccountConfig) error {
	if account.AuthType == "oauth2" {
		return c.Authenticate(sasl.NewOAuthBearerClient(&sasl.OAuthBearerOptions{
			Username: account.Username,
			Token:    account.AccessToken,
		}))
	}
	return c.Login(account.Username, account.Password)
}

func (f *IMAPFetcher) dial(account AccountConfig) (*client.Client, error) {
	addr := fmt.Sprintf("%s:%d", account.Host, account.Port)
	if account.TLS {
		return client.DialTLS(addr, &tls.Config{ServerName: account.Host, MinVersion: tls.VersionTLS12})
	}
	dialer := net.Dialer{Timeout: 15 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return client.New(conn)
}

func folderName(account AccountConfig) string {
	if strings.TrimSpace(account.Folder) == "" {
		return "INBOX"
	}
	return account.Folder
}

func wrapFolderError(folder string, err error) error {
	if err == nil {
		return nil
	}
	lowerMessage := strings.ToLower(err.Error())
	for _, marker := range []string{
		"folder not exist",
		"mailbox doesn't exist",
		"mailbox does not exist",
		"no such mailbox",
		"non-existent",
		"not found",
	} {
		if strings.Contains(lowerMessage, marker) {
			return fmt.Errorf("%w：%s", ErrFolderNotFound, folder)
		}
	}
	return err
}

func addressList(addresses []*imap.Address) string {
	return strings.Join(addressSlice(addresses), ", ")
}

func addressSlice(addresses []*imap.Address) []string {
	result := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		if addr == nil {
			continue
		}
		email := addr.MailboxName + "@" + addr.HostName
		if addr.PersonalName != "" {
			email = decodeMIMEHeader(addr.PersonalName) + " <" + email + ">"
		}
		result = append(result, email)
	}
	return result
}

func parseAddressHeader(value string) string {
	return strings.Join(parseAddressListHeader(value), ", ")
}

func parseAddressListHeader(value string) []string {
	value = strings.TrimSpace(decodeMIMEHeader(value))
	if value == "" {
		return nil
	}
	addresses, err := stdmail.ParseAddressList(value)
	if err != nil {
		return []string{value}
	}
	result := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if strings.TrimSpace(address.Name) != "" {
			result = append(result, fmt.Sprintf("%s <%s>", strings.TrimSpace(address.Name), address.Address))
			continue
		}
		result = append(result, address.Address)
	}
	return result
}

func parseBodies(raw []byte) (string, string, []FetchedAttachment) {
	raw = normalizeMIMEMessage(raw)
	reader, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil && reader == nil {
		return "", "", nil
	}

	var textBody string
	var htmlBody string
	attachments := make([]FetchedAttachment, 0)
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil && part == nil {
			break
		}
		partErr := err
		switch header := part.Header.(type) {
		case *mail.InlineHeader:
			contentType, params, _ := header.ContentType()
			body, _ := io.ReadAll(part.Body)
			contentID := cleanContentID(header.Get("Content-Id"))
			if partErr != nil && (message.IsUnknownCharset(partErr) || message.IsUnknownEncoding(partErr)) {
				body = decodeTransferBody(body, header.Get("Content-Transfer-Encoding"))
			}
			decodedBody := decodeBody(body, params)
			switch strings.ToLower(contentType) {
			case "text/plain":
				if textBody == "" {
					textBody = decodedBody
				}
			case "text/html":
				if htmlBody == "" {
					htmlBody = decodedBody
				}
			default:
				if contentID != "" {
					attachments = append(attachments, FetchedAttachment{
						Filename:    inlineFilename(contentID, contentType),
						ContentType: contentType,
						ContentID:   contentID,
						Inline:      true,
						Data:        body,
					})
				}
			}
		case *mail.AttachmentHeader:
			body, _ := io.ReadAll(part.Body)
			filename, _ := header.Filename()
			contentType, _, _ := header.ContentType()
			disposition, _, _ := header.ContentDisposition()
			contentID := cleanContentID(header.Get("Content-Id"))
			attachments = append(attachments, FetchedAttachment{
				Filename:    decodeMIMEHeader(filename),
				ContentType: contentType,
				ContentID:   contentID,
				Inline:      strings.EqualFold(disposition, "inline"),
				Data:        body,
			})
		}
	}
	return textBody, htmlBody, attachments
}

func parseMessageBodies(raw []byte) (string, string, []FetchedAttachment) {
	textBody, htmlBody, attachments := parseBodies(raw)
	if textBody == "" && htmlBody == "" {
		textBody, htmlBody = parseSinglePartBody(raw)
	}
	return textBody, htmlBody, attachments
}

func parseSinglePartBody(raw []byte) (string, string) {
	raw = normalizeMIMEMessage(raw)
	message, err := stdmail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return "", ""
	}
	body, err := io.ReadAll(message.Body)
	if err != nil {
		return "", ""
	}
	body = decodeTransferBody(body, message.Header.Get("Content-Transfer-Encoding"))
	contentType := message.Header.Get("Content-Type")
	mediaType, params, err := stdmime.ParseMediaType(contentType)
	if err != nil || strings.TrimSpace(mediaType) == "" {
		mediaType = "text/plain"
	}
	decoded := decodeBody(body, params)
	switch strings.ToLower(mediaType) {
	case "text/html":
		return "", decoded
	default:
		return decoded, ""
	}
}

func decodeTransferBody(body []byte, encoding string) []byte {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		decoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(body)))
		if err == nil {
			return decoded
		}
	case "quoted-printable":
		decoded, err := io.ReadAll(quotedprintable.NewReader(bytes.NewReader(body)))
		if err == nil {
			return decoded
		}
	}
	return body
}

func normalizeMIMEMessage(raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	firstBlank := -1
	firstBoundary := -1
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if firstBlank == -1 && trimmed == "" {
			firstBlank = index
		}
		if firstBoundary == -1 && index > 0 && strings.HasPrefix(trimmed, "--") {
			firstBoundary = index
			break
		}
	}
	if firstBoundary == -1 || (firstBlank != -1 && firstBlank < firstBoundary) {
		return raw
	}
	normalized := strings.Join(lines[:firstBoundary], "\r\n") + "\r\n\r\n" + strings.Join(lines[firstBoundary:], "\r\n")
	return []byte(normalized)
}

func decodeMIMEHeader(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	decoded, err := wordDecoder().DecodeHeader(value)
	if err != nil {
		return value
	}
	return strings.TrimSpace(decoded)
}

func decodeBody(body []byte, params map[string]string) string {
	charsetName := ""
	if params != nil {
		charsetName = params["charset"]
	}
	if strings.TrimSpace(charsetName) == "" {
		return string(body)
	}
	decoded, err := decodeCharset(body, charsetName)
	if err != nil {
		return string(body)
	}
	return decoded
}

func wordDecoder() *stdmime.WordDecoder {
	return &stdmime.WordDecoder{CharsetReader: charsetReader}
}

func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	decoder, err := charsetDecoder(charset)
	if err != nil {
		return nil, err
	}
	return transform.NewReader(input, decoder.NewDecoder()), nil
}

func decodeCharset(body []byte, charset string) (string, error) {
	decoder, err := charsetDecoder(charset)
	if err != nil {
		return "", err
	}
	decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(body), decoder.NewDecoder()))
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func charsetDecoder(charset string) (encoding.Encoding, error) {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "", "utf-8", "utf8", "us-ascii", "ascii":
		return encoding.Nop, nil
	case "gbk", "cp936", "ms936", "windows-936", "gb2312", "gb_2312", "euc-cn":
		return simplifiedchinese.GBK, nil
	case "gb18030":
		return simplifiedchinese.GB18030, nil
	case "hz-gb-2312", "hzgb2312":
		return simplifiedchinese.HZGB2312, nil
	case "big5", "big-5", "big5-hkscs":
		return traditionalchinese.Big5, nil
	case "shift_jis", "shift-jis", "sjis", "cp932", "windows-31j":
		return japanese.ShiftJIS, nil
	case "euc-jp":
		return japanese.EUCJP, nil
	case "iso-2022-jp":
		return japanese.ISO2022JP, nil
	case "euc-kr", "ks_c_5601-1987", "ks_c_5601":
		return korean.EUCKR, nil
	case "iso-8859-1", "latin1", "latin-1":
		return charmap.ISO8859_1, nil
	case "windows-1252", "cp1252":
		return charmap.Windows1252, nil
	default:
		return nil, fmt.Errorf("不支持的字符集：%s", charset)
	}
}

func cleanContentID(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "<")
	value = strings.TrimSuffix(value, ">")
	return strings.TrimSpace(value)
}

func inlineFilename(contentID, contentType string) string {
	extension := ".bin"
	switch strings.ToLower(contentType) {
	case "image/jpeg", "image/jpg":
		extension = ".jpg"
	case "image/png":
		extension = ".png"
	case "image/gif":
		extension = ".gif"
	case "image/webp":
		extension = ".webp"
	case "image/svg+xml":
		extension = ".svg"
	}
	return safeInlineName(contentID) + extension
}

func safeInlineName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "inline"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "<", "_", ">", "_", "\"", "_", "'", "_")
	return replacer.Replace(value)
}
