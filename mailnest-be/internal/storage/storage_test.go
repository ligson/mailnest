package storage

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestListDueMailAccountsFiltersEnabledAndRunningFullSync(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "mailnest.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	user, err := store.CreateUser("sync-user", "sync-user@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	due, err := store.CreateMailAccount(MailAccount{
		UserID:              user.ID,
		DisplayName:         "到期账号",
		Email:               "due@example.com",
		IMAPHost:            "imap.example.com",
		IMAPPort:            993,
		IMAPTLS:             true,
		IMAPUsername:        "due@example.com",
		IMAPPasswordEncoded: "encrypted",
		PollIntervalMinutes: 10,
		Enabled:             true,
	})
	if err != nil {
		t.Fatalf("create due account: %v", err)
	}
	if _, err := store.CreateMailAccount(MailAccount{
		UserID:              user.ID,
		DisplayName:         "停用账号",
		Email:               "disabled@example.com",
		IMAPHost:            "imap.example.com",
		IMAPPort:            993,
		IMAPTLS:             true,
		IMAPUsername:        "disabled@example.com",
		IMAPPasswordEncoded: "encrypted",
		PollIntervalMinutes: 10,
		Enabled:             false,
	}); err != nil {
		t.Fatalf("create disabled account: %v", err)
	}
	running, err := store.CreateMailAccount(MailAccount{
		UserID:              user.ID,
		DisplayName:         "全量同步中",
		Email:               "running@example.com",
		IMAPHost:            "imap.example.com",
		IMAPPort:            993,
		IMAPTLS:             true,
		IMAPUsername:        "running@example.com",
		IMAPPasswordEncoded: "encrypted",
		PollIntervalMinutes: 10,
		Enabled:             true,
	})
	if err != nil {
		t.Fatalf("create running account: %v", err)
	}
	if err := store.StartMailAccountFullSync(user.ID, running.ID, 0); err != nil {
		t.Fatalf("mark full sync running: %v", err)
	}

	accounts, err := store.ListDueMailAccounts(20)
	if err != nil {
		t.Fatalf("list due accounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected one due account, got %#v", accounts)
	}
	if accounts[0].ID != due.ID {
		t.Fatalf("expected due account %d, got %d", due.ID, accounts[0].ID)
	}
}

func TestListDueMailAccountsSkipsRecentlySynced(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "mailnest.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	user, err := store.CreateUser("recent-user", "recent-user@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	account, err := store.CreateMailAccount(MailAccount{
		UserID:              user.ID,
		DisplayName:         "刚同步账号",
		Email:               "recent@example.com",
		IMAPHost:            "imap.example.com",
		IMAPPort:            993,
		IMAPTLS:             true,
		IMAPUsername:        "recent@example.com",
		IMAPPasswordEncoded: "encrypted",
		PollIntervalMinutes: 10,
		Enabled:             true,
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := store.UpdateMailAccountSyncStatus(user.ID, account.ID, "success", ""); err != nil {
		t.Fatalf("update sync status: %v", err)
	}

	accounts, err := store.ListDueMailAccounts(20)
	if err != nil {
		t.Fatalf("list due accounts: %v", err)
	}
	if len(accounts) != 0 {
		t.Fatalf("expected no due accounts, got %#v", accounts)
	}
}

func TestUpsertContactSeenDoesNotOverwriteManualProfile(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "mailnest.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	user, err := store.CreateUser("contact-user", "contact-user@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	contact, err := store.UpsertContactSeen(CreateContactParams{
		UserID:      user.ID,
		Email:       "Friend@Example.com",
		DisplayName: "邮件姓名",
		Source:      "auto",
		SeenAt:      sql.NullTime{Time: time.Now().Add(-time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("upsert auto contact: %v", err)
	}
	contact.DisplayName = sql.NullString{String: "手工姓名", Valid: true}
	contact.Nickname = sql.NullString{String: "老友", Valid: true}
	contact.Source = "manual"
	contact, err = store.UpdateContact(contact)
	if err != nil {
		t.Fatalf("update manual contact: %v", err)
	}

	updated, err := store.UpsertContactSeen(CreateContactParams{
		UserID:      user.ID,
		Email:       "friend@example.com",
		DisplayName: "新的邮件姓名",
		Source:      "auto",
		SeenAt:      sql.NullTime{Time: time.Now(), Valid: true},
	})
	if err != nil {
		t.Fatalf("upsert seen contact: %v", err)
	}
	if updated.ID != contact.ID {
		t.Fatalf("expected same contact id %d, got %d", contact.ID, updated.ID)
	}
	if updated.DisplayName.String != "手工姓名" || updated.Nickname.String != "老友" || updated.Source != "manual" {
		t.Fatalf("expected manual profile to remain, got %#v", updated)
	}
}

func TestListMailMessagesByQueryUsesSummaryFields(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "mailnest.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	user, err := store.CreateUser("message-user", "message-user@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	account, err := store.CreateMailAccount(MailAccount{
		UserID:              user.ID,
		DisplayName:         "列表账号",
		Email:               "message-user@example.com",
		IMAPHost:            "imap.example.com",
		IMAPPort:            993,
		IMAPTLS:             true,
		IMAPUsername:        "message-user@example.com",
		IMAPPasswordEncoded: "encrypted",
		PollIntervalMinutes: 10,
		Enabled:             true,
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	created, inserted, err := store.InsertMailMessageIfNew(CreateMailMessageParams{
		UserID:     user.ID,
		AccountID:  account.ID,
		Folder:     "INBOX",
		IMAPUID:    "1",
		Subject:    "摘要查询",
		FromAddr:   "sender@example.com",
		ToAddrs:    "message-user@example.com",
		ReceivedAt: sql.NullTime{Time: time.Now(), Valid: true},
		SearchText: "正文搜索索引不应该出现在列表结果里",
	})
	if err != nil {
		t.Fatalf("insert message: %v", err)
	}
	if !inserted {
		t.Fatal("expected message to be inserted")
	}

	messages, total, err := store.ListMailMessagesByQuery(ListMailMessagesQuery{
		UserID:       user.ID,
		SystemFolder: "inbox",
		Limit:        20,
		SummaryOnly:  true,
	})
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if total != 1 || len(messages) != 1 {
		t.Fatalf("expected one listed message, total=%d messages=%#v", total, messages)
	}
	if messages[0].SearchText.Valid {
		t.Fatalf("expected list result to skip search text, got %#v", messages[0].SearchText)
	}

	detail, err := store.FindMailMessageByID(user.ID, created.ID)
	if err != nil {
		t.Fatalf("find message detail: %v", err)
	}
	if detail.SearchText.String != "正文搜索索引不应该出现在列表结果里" {
		t.Fatalf("expected detail query to include search text, got %#v", detail.SearchText)
	}
}
