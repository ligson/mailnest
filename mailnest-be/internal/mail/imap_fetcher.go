package mail

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-sasl"
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
	return err
}

func (f *IMAPFetcher) FetchInbox(account AccountConfig) ([]FetchedMessage, error) {
	c, err := f.dial(account)
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	if err := authenticate(c, account); err != nil {
		return nil, err
	}

	mbox, err := c.Select(folderName(account), false)
	if err != nil {
		return nil, err
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
		item := FetchedMessage{
			UID: fmt.Sprint(msg.Uid),
		}
		if msg.Envelope != nil {
			item.MessageID = msg.Envelope.MessageId
			item.Subject = msg.Envelope.Subject
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
			textBody, htmlBody, attachments := parseBodies(raw)
			item.TextBody = textBody
			item.HTMLBody = htmlBody
			item.Attachments = attachments
		}
		fetched = append(fetched, item)
	}
	if err := <-done; err != nil {
		return nil, err
	}

	return fetched, nil
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
			email = addr.PersonalName + " <" + email + ">"
		}
		result = append(result, email)
	}
	return result
}

func parseBodies(raw []byte) (string, string, []FetchedAttachment) {
	reader, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
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
		if err != nil {
			break
		}
		switch header := part.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, _ := header.ContentType()
			body, _ := io.ReadAll(part.Body)
			contentID := cleanContentID(header.Get("Content-Id"))
			switch strings.ToLower(contentType) {
			case "text/plain":
				if textBody == "" {
					textBody = string(body)
				}
			case "text/html":
				if htmlBody == "" {
					htmlBody = string(body)
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
				Filename:    filename,
				ContentType: contentType,
				ContentID:   contentID,
				Inline:      strings.EqualFold(disposition, "inline"),
				Data:        body,
			})
		}
	}
	return textBody, htmlBody, attachments
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
