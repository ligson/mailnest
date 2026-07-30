package mail

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"mailnest-be/internal/storage"
)

func TestParseTimeAcceptsDateWithTrailingTimezoneComment(t *testing.T) {
	parsed := parseTime("Mon, 8 Sep 2025 19:03:56 +0800 (GMT+08:00)")
	if !parsed.Valid {
		t.Fatal("expected mail Date with trailing timezone comment to parse")
	}
	if got := parsed.Time.Format("2006-01-02 15:04:05 -0700"); got != "2025-09-08 19:03:56 +0800" {
		t.Fatalf("unexpected parsed time: %s", got)
	}
}

func TestRepairMissingSentAtUsesOriginalDateHeader(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "mailnest.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	user, err := store.CreateUser("date-repair", "date-repair@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	account, err := store.CreateMailAccount(storage.MailAccount{
		UserID:              user.ID,
		DisplayName:         "日期修复邮箱",
		Email:               "date-repair@example.com",
		IMAPHost:            "imap.example.com",
		IMAPPort:            993,
		IMAPTLS:             true,
		IMAPUsername:        "date-repair@example.com",
		IMAPPasswordEncoded: "encrypted",
		PollIntervalMinutes: 10,
		Enabled:             true,
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	dataDir := t.TempDir()
	raw := []byte("From: Sender <sender@example.com>\r\nTo: Reader <reader@example.com>\r\nSubject: Date Repair\r\nMessage-ID: <date-repair@example.com>\r\nDate: Mon, 8 Sep 2025 19:03:56 +0800 (GMT+08:00)\r\n\r\nHello date repair.")
	rawPath := filepath.Join(dataDir, "date-repair.eml")
	if err := os.WriteFile(rawPath, raw, 0o600); err != nil {
		t.Fatalf("write raw eml: %v", err)
	}
	message, inserted, err := store.InsertMailMessageIfNew(storage.CreateMailMessageParams{
		UserID:     user.ID,
		AccountID:  account.ID,
		Folder:     "INBOX",
		IMAPUID:    "eml-date-repair",
		MessageID:  "<date-repair@example.com>",
		Subject:    "Date Repair",
		FromAddr:   "sender@example.com",
		ToAddrs:    "reader@example.com",
		ReceivedAt: sql.NullTime{},
		RawPath:    rawPath,
	})
	if err != nil {
		t.Fatalf("insert message: %v", err)
	}
	if !inserted || message.SentAt.Valid {
		t.Fatalf("expected inserted message with empty sentAt, inserted=%t message=%#v", inserted, message)
	}

	service := NewService(store, nil, nil, dataDir, "secret")
	result, err := service.RepairMissingSentAt(user.ID, account.ID, true, 10)
	if err != nil {
		t.Fatalf("repair missing sent_at: %v", err)
	}
	if result.Checked != 1 || result.Repaired != 1 || result.NoValidDate != 0 {
		t.Fatalf("unexpected repair result: %#v", result)
	}
	repaired, err := store.FindMailMessageByID(user.ID, message.ID)
	if err != nil {
		t.Fatalf("find repaired message: %v", err)
	}
	if !repaired.SentAt.Valid || repaired.SentAt.Time.Format("2006-01-02 15:04:05 -0700") != "2025-09-08 19:03:56 +0800" {
		t.Fatalf("expected repaired sentAt from Date header, got %#v", repaired.SentAt)
	}
}
