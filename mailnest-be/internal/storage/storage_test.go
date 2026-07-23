package storage

import (
	"database/sql"
	"errors"
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

func TestSaveMailDraftLifecycleAndIsolation(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "mailnest.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	user, err := store.CreateUser("draft-user", "draft-user@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	otherUser, err := store.CreateUser("other-draft-user", "other-draft-user@example.com", "hash")
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}
	account, err := store.CreateMailAccount(MailAccount{
		UserID:              user.ID,
		DisplayName:         "草稿账号",
		Email:               "draft-user@example.com",
		IMAPHost:            "imap.example.com",
		IMAPPort:            993,
		IMAPTLS:             true,
		IMAPUsername:        "draft-user@example.com",
		IMAPPasswordEncoded: "encrypted",
		PollIntervalMinutes: 10,
		Enabled:             true,
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	message, _, err := store.InsertMailMessageIfNew(CreateMailMessageParams{
		UserID:    user.ID,
		AccountID: account.ID,
		Folder:    "INBOX",
		IMAPUID:   "draft-source",
		Subject:   "来源邮件",
		FromAddr:  "sender@example.com",
		ToAddrs:   "draft-user@example.com",
	})
	if err != nil {
		t.Fatalf("insert source message: %v", err)
	}

	draft, err := store.SaveMailDraft(SaveMailDraftParams{
		UserID:                   user.ID,
		AccountID:                account.ID,
		ComposeMode:              "reply",
		SourceMessageID:          sql.NullInt64{Int64: message.ID, Valid: true},
		ToAddrsJSON:              `["sender@example.com"]`,
		CCAddrsJSON:              `[]`,
		BCCAddrsJSON:             `[]`,
		Subject:                  "Re: 来源邮件",
		TextBody:                 "草稿正文",
		HTMLBody:                 "<p>草稿正文</p>",
		ForwardAttachmentIDsJSON: `[]`,
		LocalAttachmentNamesJSON: `["report.pdf"]`,
	})
	if err != nil {
		t.Fatalf("save draft: %v", err)
	}
	if draft.ID == 0 || draft.Subject != "Re: 来源邮件" || draft.LocalAttachmentNamesJSON != `["report.pdf"]` {
		t.Fatalf("unexpected draft: %#v", draft)
	}

	draft.Subject = "更新后的草稿"
	updated, err := store.SaveMailDraft(SaveMailDraftParams{
		ID:                       draft.ID,
		UserID:                   user.ID,
		AccountID:                account.ID,
		ComposeMode:              "reply",
		SourceMessageID:          sql.NullInt64{Int64: message.ID, Valid: true},
		ToAddrsJSON:              `["sender@example.com"]`,
		CCAddrsJSON:              `[]`,
		BCCAddrsJSON:             `[]`,
		Subject:                  draft.Subject,
		TextBody:                 draft.TextBody,
		HTMLBody:                 draft.HTMLBody,
		ForwardAttachmentIDsJSON: `[]`,
		LocalAttachmentNamesJSON: draft.LocalAttachmentNamesJSON,
	})
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}
	if updated.ID != draft.ID || updated.Subject != "更新后的草稿" {
		t.Fatalf("unexpected updated draft: %#v", updated)
	}

	drafts, total, err := store.ListMailDrafts(ListMailDraftsQuery{UserID: user.ID, Limit: 20})
	if err != nil {
		t.Fatalf("list drafts: %v", err)
	}
	if total != 1 || len(drafts) != 1 || drafts[0].ID != draft.ID {
		t.Fatalf("expected one draft, total=%d drafts=%#v", total, drafts)
	}
	if _, err := store.FindMailDraftByID(otherUser.ID, draft.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected other user not to see draft, got %v", err)
	}
	if err := store.DeleteMailDraft(user.ID, draft.ID); err != nil {
		t.Fatalf("delete draft: %v", err)
	}
	if _, err := store.FindMailDraftByID(user.ID, draft.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected deleted draft not found, got %v", err)
	}
}
