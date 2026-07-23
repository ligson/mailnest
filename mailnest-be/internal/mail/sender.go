package mail

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	netmail "net/mail"
	"net/smtp"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type SMTPConfig struct {
	Email       string
	DisplayName string
	Host        string
	Port        int
	TLS         bool
	StartTLS    bool
	Username    string
	Password    string
}

type OutgoingMessage struct {
	DraftID              int64
	FromName             string
	From                 string
	To                   []string
	CC                   []string
	BCC                  []string
	Subject              string
	TextBody             string
	HTMLBody             string
	InReplyTo            string
	References           []string
	ComposeMode          string
	SourceMessageID      int64
	ForwardAttachmentIDs []int64
	Attachments          []OutgoingAttachment
}

type OutgoingAttachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

type SendResult struct {
	MessageID string
	Raw       string
	SentAt    time.Time
}

type Sender interface {
	Send(account SMTPConfig, message OutgoingMessage) (SendResult, error)
}

type SMTPSender struct{}

func (s *SMTPSender) Send(account SMTPConfig, message OutgoingMessage) (SendResult, error) {
	account.Host = strings.TrimSpace(account.Host)
	if account.Host == "" || account.Port <= 0 {
		return SendResult{}, errors.New("SMTP 发信配置不完整")
	}
	if strings.TrimSpace(account.Email) == "" {
		return SendResult{}, errors.New("发件邮箱不能为空")
	}
	recipients, err := envelopeRecipients(message)
	if err != nil {
		return SendResult{}, err
	}
	if len(recipients) == 0 {
		return SendResult{}, errors.New("至少需要一个收件人")
	}

	sentAt := time.Now()
	messageID := newMessageID(account.Email, sentAt)
	raw, err := buildSMTPMessage(account, message, messageID, sentAt)
	if err != nil {
		return SendResult{}, err
	}
	if err := sendRawSMTP(account, account.Email, recipients, []byte(raw)); err != nil {
		return SendResult{}, err
	}
	return SendResult{MessageID: messageID, Raw: raw, SentAt: sentAt}, nil
}

type FakeSender struct {
	Err      error
	Account  SMTPConfig
	Message  OutgoingMessage
	Result   SendResult
	Messages []OutgoingMessage
}

func (f *FakeSender) Send(account SMTPConfig, message OutgoingMessage) (SendResult, error) {
	f.Account = account
	f.Message = message
	f.Messages = append(f.Messages, message)
	if f.Err != nil {
		return SendResult{}, f.Err
	}
	if strings.TrimSpace(f.Result.MessageID) != "" {
		return f.Result, nil
	}
	sentAt := time.Now()
	raw, err := buildSMTPMessage(account, message, "<fake-message@mailnest.local>", sentAt)
	if err != nil {
		return SendResult{}, err
	}
	f.Result = SendResult{MessageID: "<fake-message@mailnest.local>", Raw: raw, SentAt: sentAt}
	return f.Result, nil
}

func buildSMTPMessage(account SMTPConfig, message OutgoingMessage, messageID string, sentAt time.Time) (string, error) {
	from := strings.TrimSpace(message.From)
	if from == "" {
		from = account.Email
	}
	fromAddress := &netmail.Address{Name: firstNonEmpty(message.FromName, account.DisplayName), Address: from}
	to, err := parseAddressList(message.To)
	if err != nil {
		return "", fmt.Errorf("收件人格式不正确：%w", err)
	}
	cc, err := parseAddressList(message.CC)
	if err != nil {
		return "", fmt.Errorf("抄送人格式不正确：%w", err)
	}
	if _, err := parseAddressList(message.BCC); err != nil {
		return "", fmt.Errorf("密送人格式不正确：%w", err)
	}

	headers := map[string]string{
		"From":         fromAddress.String(),
		"To":           joinAddresses(to),
		"Subject":      mime.QEncoding.Encode("UTF-8", strings.TrimSpace(message.Subject)),
		"Date":         sentAt.Format(time.RFC1123Z),
		"Message-ID":   messageID,
		"MIME-Version": "1.0",
	}
	if len(cc) > 0 {
		headers["Cc"] = joinAddresses(cc)
	}
	if strings.TrimSpace(message.InReplyTo) != "" {
		headers["In-Reply-To"] = strings.TrimSpace(message.InReplyTo)
	}
	if refs := normalizedReferences(message.References); len(refs) > 0 {
		headers["References"] = strings.Join(refs, " ")
	}

	alternativeBoundary := "mailnest-alt-" + randomHex(12)
	var body bytes.Buffer
	writeHeaders(&body, headers)
	if len(message.Attachments) > 0 {
		mixedBoundary := "mailnest-mixed-" + randomHex(12)
		body.WriteString("Content-Type: multipart/mixed; boundary=\"" + mixedBoundary + "\"\r\n\r\n")
		body.WriteString("--" + mixedBoundary + "\r\n")
		writeBodyPart(&body, alternativeBoundary, message.TextBody, message.HTMLBody)
		for _, attachment := range message.Attachments {
			writeAttachmentPart(&body, mixedBoundary, attachment)
		}
		body.WriteString("--" + mixedBoundary + "--\r\n")
		return body.String(), nil
	}
	writeBodyPart(&body, alternativeBoundary, message.TextBody, message.HTMLBody)
	return body.String(), nil
}

func writeBodyPart(body *bytes.Buffer, boundary, textBody, htmlBody string) {
	if strings.TrimSpace(htmlBody) != "" {
		body.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n")
		writeAlternativePart(body, boundary, "text/plain; charset=UTF-8", textBodyOrFallback(textBody, htmlBody))
		writeAlternativePart(body, boundary, "text/html; charset=UTF-8", htmlBody)
		body.WriteString("--" + boundary + "--\r\n")
	} else {
		body.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		body.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		writeQuotedPrintable(body, textBody)
	}
}

func sendRawSMTP(account SMTPConfig, from string, recipients []string, raw []byte) error {
	address := net.JoinHostPort(account.Host, fmt.Sprint(account.Port))
	var conn net.Conn
	var err error
	if account.TLS {
		conn, err = tls.Dial("tcp", address, &tls.Config{ServerName: account.Host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = net.Dial("tcp", address)
	}
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, account.Host)
	if err != nil {
		return err
	}
	defer client.Close()

	if !account.TLS && account.StartTLS {
		if err := client.StartTLS(&tls.Config{ServerName: account.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if strings.TrimSpace(account.Username) != "" || strings.TrimSpace(account.Password) != "" {
		if err := client.Auth(smtp.PlainAuth("", account.Username, account.Password, account.Host)); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(raw); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func envelopeRecipients(message OutgoingMessage) ([]string, error) {
	addresses := make([]string, 0)
	for _, group := range [][]string{message.To, message.CC, message.BCC} {
		parsed, err := parseAddressList(group)
		if err != nil {
			return nil, err
		}
		for _, address := range parsed {
			addresses = append(addresses, strings.TrimSpace(address.Address))
		}
	}
	sort.Strings(addresses)
	return addresses, nil
}

func parseAddressList(values []string) ([]*netmail.Address, error) {
	joined := strings.TrimSpace(strings.Join(nonEmptyStrings(values), ", "))
	if joined == "" {
		return nil, nil
	}
	return netmail.ParseAddressList(joined)
}

func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func joinAddresses(addresses []*netmail.Address) string {
	values := make([]string, 0, len(addresses))
	for _, address := range addresses {
		values = append(values, address.String())
	}
	return strings.Join(values, ", ")
}

func writeHeaders(body *bytes.Buffer, headers map[string]string) {
	for _, key := range []string{"From", "To", "Cc", "Subject", "Date", "Message-ID", "In-Reply-To", "References", "MIME-Version"} {
		value := strings.TrimSpace(headers[key])
		if value != "" {
			body.WriteString(key + ": " + value + "\r\n")
		}
	}
}

func normalizedReferences(values []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Fields(strings.TrimSpace(value)) {
			if !strings.HasPrefix(part, "<") || !strings.HasSuffix(part, ">") {
				continue
			}
			key := strings.ToLower(part)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, part)
		}
	}
	return out
}

func writeAlternativePart(body *bytes.Buffer, boundary, contentType, content string) {
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Type: " + contentType + "\r\n")
	body.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	writeQuotedPrintable(body, content)
	body.WriteString("\r\n")
}

func writeAttachmentPart(body *bytes.Buffer, boundary string, attachment OutgoingAttachment) {
	filename := strings.TrimSpace(attachment.Filename)
	if filename == "" {
		filename = "attachment"
	}
	contentType := strings.TrimSpace(attachment.ContentType)
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Type: " + mime.FormatMediaType(contentType, map[string]string{"name": filename}) + "\r\n")
	body.WriteString("Content-Disposition: " + mime.FormatMediaType("attachment", map[string]string{"filename": filename}) + "\r\n")
	body.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	writeBase64Wrapped(body, attachment.Data)
	body.WriteString("\r\n")
}

func writeBase64Wrapped(body *bytes.Buffer, data []byte) {
	encoder := base64.NewEncoder(base64.StdEncoding, newBase64LineWriter(body))
	_, _ = encoder.Write(data)
	_ = encoder.Close()
}

type base64LineWriter struct {
	writer *bytes.Buffer
	line   int
}

func newBase64LineWriter(writer *bytes.Buffer) io.Writer {
	return &base64LineWriter{writer: writer}
}

func (w *base64LineWriter) Write(data []byte) (int, error) {
	for _, b := range data {
		if w.line == 76 {
			w.writer.WriteString("\r\n")
			w.line = 0
		}
		w.writer.WriteByte(b)
		w.line++
	}
	return len(data), nil
}

func writeQuotedPrintable(body *bytes.Buffer, content string) {
	writer := quotedprintable.NewWriter(body)
	_, _ = writer.Write([]byte(content))
	_ = writer.Close()
}

func textBodyOrFallback(textBody, htmlBody string) string {
	if strings.TrimSpace(textBody) != "" {
		return textBody
	}
	return stripHTMLTags(htmlBody)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func newMessageID(email string, sentAt time.Time) string {
	domain := "mailnest.local"
	if _, after, ok := strings.Cut(email, "@"); ok && strings.TrimSpace(after) != "" {
		domain = strings.TrimSpace(after)
	}
	return fmt.Sprintf("<%d.%s@%s>", sentAt.UnixNano(), randomHex(8), domain)
}

func randomHex(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
