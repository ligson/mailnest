package mail

import (
	"os"
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

func TestComposeContextReplyAndForward(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "mailnest.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	user, err := store.CreateUser("reader", "reader@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	account, err := store.CreateMailAccount(storage.MailAccount{
		UserID:              user.ID,
		DisplayName:         "读信人",
		Email:               "reader@example.com",
		IMAPHost:            "imap.example.com",
		IMAPPort:            993,
		IMAPTLS:             true,
		IMAPUsername:        "reader@example.com",
		IMAPPasswordEncoded: "encoded",
		SentFolder:          "Sent",
		PollIntervalMinutes: 10,
		Enabled:             true,
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	service := NewServiceWithSender(store, &FakeFetcher{}, &FakeSender{}, nil, t.TempDir(), "secret")
	if _, err := store.CreateContact(storage.CreateContactParams{
		UserID:      user.ID,
		Email:       "licongb@yonyou.com",
		DisplayName: "李聪",
		Nickname:    "李聪通讯录",
		Source:      "manual",
	}); err != nil {
		t.Fatalf("create sender contact: %v", err)
	}
	if _, err := store.CreateContact(storage.CreateContactParams{
		UserID:      user.ID,
		Email:       "wangceb@yonyou.com",
		DisplayName: "王策彬通讯录",
		Source:      "manual",
	}); err != nil {
		t.Fatalf("create cc contact: %v", err)
	}

	mailMsg, _, err := store.InsertMailMessageIfNew(storage.CreateMailMessageParams{
		UserID:       user.ID,
		AccountID:    account.ID,
		Folder:       "INBOX",
		IMAPUID:      "uid-1",
		MessageID:    "<origin@example.com>",
		Subject:      "项目进展",
		FromAddr:     "=?utf-8?q?=E6=9D=8E=E8=81=AA?= <licongb@yonyou.com>",
		ToAddrs:      "reader@example.com, =?utf-8?q?=E7=8E=8B=E7=AD=96=E5=BD=AC?= <wangceb@yonyou.com>",
		CCAddrs:      "=?utf-8?q?=E5=AD=99=E6=97=AD=E4=B8=9C?= <sunxd@yonyou.com>, reader@example.com",
		SearchText:   "项目进展",
		RawPath:      "raw",
		TextBodyPath: "text",
		HTMLBodyPath: "html",
		InReplyTo:    "<root@example.com>",
		References:   "<root@example.com> <parent@example.com>",
	})
	if err != nil {
		t.Fatalf("insert source message: %v", err)
	}
	attachmentPath := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(attachmentPath, []byte("report-body"), 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	if _, err := store.CreateMailAttachment(storage.CreateMailAttachmentParams{
		UserID:      user.ID,
		MessageID:   mailMsg.ID,
		Filename:    "report.pdf",
		ContentType: "application/pdf",
		Size:        int64(len("report-body")),
		FilePath:    attachmentPath,
	}); err != nil {
		t.Fatalf("insert attachment: %v", err)
	}

	replyCtx, err := service.GetComposeContext(user.ID, mailMsg.ID, "replyAll")
	if err != nil {
		t.Fatalf("reply context: %v", err)
	}
	if replyCtx.Mode != "replyAll" {
		t.Fatalf("expected replyAll mode, got %#v", replyCtx)
	}
	if len(replyCtx.To) != 1 || replyCtx.To[0] != "李聪通讯录 <licongb@yonyou.com>" {
		t.Fatalf("expected reply to prefer contact nickname, got %#v", replyCtx.To)
	}
	joinedCC := strings.Join(replyCtx.CC, ",")
	if len(replyCtx.CC) != 2 || !strings.Contains(joinedCC, "王策彬通讯录 <wangceb@yonyou.com>") || !strings.Contains(joinedCC, "孙旭东 <sunxd@yonyou.com>") || strings.Contains(joinedCC, "=?") {
		t.Fatalf("expected reply all cc to keep other participants and exclude own addresses, got %#v", replyCtx.CC)
	}
	if !strings.Contains(replyCtx.TextBody, "在") || !strings.Contains(replyCtx.TextBody, "写道") {
		t.Fatalf("expected reply quote text, got %q", replyCtx.TextBody)
	}

	singleReplyCtx, err := service.GetComposeContext(user.ID, mailMsg.ID, "reply")
	if err != nil {
		t.Fatalf("single reply context: %v", err)
	}
	if len(singleReplyCtx.To) != 1 || singleReplyCtx.To[0] != "李聪通讯录 <licongb@yonyou.com>" {
		t.Fatalf("expected single reply to prefer contact nickname, got %#v", singleReplyCtx.To)
	}

	forwardCtx, err := service.GetComposeContext(user.ID, mailMsg.ID, "forward")
	if err != nil {
		t.Fatalf("forward context: %v", err)
	}
	if forwardCtx.Mode != "forward" {
		t.Fatalf("expected forward mode, got %#v", forwardCtx)
	}
	if len(forwardCtx.ForwardAttachments) != 1 || !forwardCtx.ForwardAttachments[0].Selected {
		t.Fatalf("expected forward attachment selected, got %#v", forwardCtx.ForwardAttachments)
	}
	if !strings.Contains(forwardCtx.HTMLBody, "转发邮件") ||
		!strings.Contains(forwardCtx.HTMLBody, "李聪通讯录 &lt;licongb@yonyou.com&gt;") ||
		!strings.Contains(forwardCtx.HTMLBody, "王策彬通讯录 &lt;wangceb@yonyou.com&gt;") ||
		strings.Contains(forwardCtx.HTMLBody, "=?utf-8") {
		t.Fatalf("expected forward html body, got %q", forwardCtx.HTMLBody)
	}
}

func TestSendReplyWritesThreadHeadersAndForwardAttachments(t *testing.T) {
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
		PollIntervalMinutes: 10,
		Enabled:             true,
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	source, _, err := store.InsertMailMessageIfNew(storage.CreateMailMessageParams{
		UserID:       user.ID,
		AccountID:    account.ID,
		Folder:       "INBOX",
		IMAPUID:      "uid-2",
		MessageID:    "<source@example.com>",
		Subject:      "原邮件",
		FromAddr:     "other@example.com",
		ToAddrs:      "sender@example.com",
		SearchText:   "原邮件",
		RawPath:      "raw",
		TextBodyPath: "text",
		HTMLBodyPath: "html",
	})
	if err != nil {
		t.Fatalf("insert source message: %v", err)
	}
	attachmentDir := t.TempDir()
	attachmentPath := filepath.Join(attachmentDir, "forward.txt")
	if err := os.WriteFile(attachmentPath, []byte("forward-body"), 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	attachment, err := store.CreateMailAttachment(storage.CreateMailAttachmentParams{
		UserID:      user.ID,
		MessageID:   source.ID,
		Filename:    "forward.txt",
		ContentType: "text/plain",
		Size:        int64(len("forward-body")),
		FilePath:    attachmentPath,
	})
	if err != nil {
		t.Fatalf("add attachment: %v", err)
	}

	fakeSender := &FakeSender{}
	service := NewServiceWithSender(store, &FakeFetcher{}, fakeSender, nil, t.TempDir(), secret)
	sent, err := service.SendMessage(user.ID, account.ID, OutgoingMessage{
		ComposeMode:          "replyAll",
		SourceMessageID:      source.ID,
		To:                   []string{"other@example.com"},
		Subject:              "Re: 原邮件",
		TextBody:             "收到",
		HTMLBody:             "<p>收到</p>",
		ForwardAttachmentIDs: []int64{attachment.ID},
	})
	if err != nil {
		t.Fatalf("send reply: %v", err)
	}
	if !strings.Contains(fakeSender.Message.InReplyTo, "<source@example.com>") {
		t.Fatalf("expected in-reply-to header, got %#v", fakeSender.Message)
	}
	if len(fakeSender.Message.References) == 0 || fakeSender.Message.References[len(fakeSender.Message.References)-1] != "<source@example.com>" {
		t.Fatalf("expected references to include source message, got %#v", fakeSender.Message.References)
	}
	if len(fakeSender.Message.Attachments) != 1 || fakeSender.Message.Attachments[0].Filename != "forward.txt" {
		t.Fatalf("expected forwarded attachment to be re-packed, got %#v", fakeSender.Message.Attachments)
	}
	if sent.Folder != "Sent" {
		t.Fatalf("expected sent folder mail saved, got %#v", sent)
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
