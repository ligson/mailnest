package mail

import (
	"path/filepath"
	"strings"
	"testing"

	"mailnest-be/internal/crypto"
	"mailnest-be/internal/storage"
)

func TestSendMessageUsesSMTPAndSavesSentMessage(t *testing.T) {
	const secret = "test-credential-secret"

	store, err := storage.Open(filepath.Join(t.TempDir(), "mailnest.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	user, err := store.CreateUser("sender", "sender@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	imapPassword, err := crypto.EncryptString("imap-pass", secret)
	if err != nil {
		t.Fatalf("encrypt imap password: %v", err)
	}
	smtpPassword, err := crypto.EncryptString("smtp-pass", secret)
	if err != nil {
		t.Fatalf("encrypt smtp password: %v", err)
	}
	account, err := store.CreateMailAccount(storage.MailAccount{
		UserID:              user.ID,
		DisplayName:         "发件人",
		Email:               "sender@example.com",
		IMAPHost:            "imap.example.com",
		IMAPPort:            993,
		IMAPTLS:             true,
		IMAPUsername:        "sender@example.com",
		IMAPPasswordEncoded: imapPassword,
		SMTPHost:            "smtp.example.com",
		SMTPPort:            587,
		SMTPStartTLS:        true,
		SMTPUsername:        "smtp-user",
		SMTPPasswordEncoded: smtpPassword,
		SentFolder:          "Sent",
		SignatureHTML:       "<p>发件人签名</p>",
		PollIntervalMinutes: 10,
		Enabled:             true,
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	fakeSender := &FakeSender{}
	service := NewServiceWithSender(store, &FakeFetcher{}, fakeSender, nil, t.TempDir(), secret)
	message, err := service.SendMessage(user.ID, account.ID, OutgoingMessage{
		To:       []string{"好友 <friend@example.com>"},
		CC:       []string{"copy@example.com"},
		BCC:      []string{"Hidden <hidden@example.com>"},
		Subject:  "测试发信",
		TextBody: "这是一封测试邮件",
		Attachments: []OutgoingAttachment{
			{
				Filename:    "report.txt",
				ContentType: "text/plain",
				Data:        []byte("attachment-body"),
			},
		},
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}

	if fakeSender.Account.Host != "smtp.example.com" || fakeSender.Account.Username != "smtp-user" || fakeSender.Account.Password != "smtp-pass" {
		t.Fatalf("expected decrypted smtp config, got %#v", fakeSender.Account)
	}
	if message.Folder != "Sent" || message.Subject.String != "测试发信" {
		t.Fatalf("expected saved sent message, got %#v", message)
	}
	if message.ToAddrs.String != "好友 <friend@example.com>" || message.CCAddrs.String != "copy@example.com" {
		t.Fatalf("expected saved recipients, got to=%q cc=%q", message.ToAddrs.String, message.CCAddrs.String)
	}
	if !message.HasAttachments {
		t.Fatalf("expected saved sent message to have attachments, got %#v", message)
	}
	if len(fakeSender.Message.Attachments) != 1 || fakeSender.Message.Attachments[0].Filename != "report.txt" {
		t.Fatalf("expected smtp sender to receive attachment, got %#v", fakeSender.Message.Attachments)
	}
	if !strings.Contains(fakeSender.Result.Raw, "Content-Type: multipart/mixed") || !strings.Contains(fakeSender.Result.Raw, "report.txt") {
		t.Fatalf("expected smtp raw message to include attachment, got %q", fakeSender.Result.Raw)
	}

	attachments, err := store.ListMailAttachments(user.ID, message.ID)
	if err != nil {
		t.Fatalf("list saved attachments: %v", err)
	}
	if len(attachments) != 1 || attachments[0].Filename != "report.txt" || attachments[0].Size != int64(len("attachment-body")) {
		t.Fatalf("expected sent attachment metadata, got %#v", attachments)
	}
	loadedAccount, err := store.FindMailAccountByID(user.ID, account.ID)
	if err != nil {
		t.Fatalf("load account: %v", err)
	}
	if loadedAccount.SignatureHTML != "<p>发件人签名</p>" {
		t.Fatalf("expected signature html to be saved, got %q", loadedAccount.SignatureHTML)
	}

	contacts, total, err := store.ListContacts(storage.ListContactsQuery{UserID: user.ID, Limit: 20})
	if err != nil {
		t.Fatalf("list contacts: %v", err)
	}
	if total < 4 {
		t.Fatalf("expected sender/to/cc/bcc contacts, got total=%d contacts=%#v", total, contacts)
	}
	if !hasContactEmail(contacts, "hidden@example.com") {
		t.Fatalf("expected bcc contact to be upserted, got %#v", contacts)
	}
}

func hasContactEmail(contacts []storage.Contact, email string) bool {
	for _, contact := range contacts {
		if contact.Email == email {
			return true
		}
	}
	return false
}
