package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSearchOutboxTransactionAndLegacyMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mail.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	user, err := s.CreateUser("outbox", "outbox@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	account, err := s.CreateMailAccount(MailAccount{UserID: user.ID, DisplayName: "测试", Email: "test@example.com", IMAPHost: "example.com", IMAPPort: 993, IMAPUsername: "test", IMAPPasswordEncoded: "encrypted", PollIntervalMinutes: 10, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	p := CreateMailMessageParams{UserID: user.ID, AccountID: account.ID, Folder: "INBOX", IMAPUID: "1", Subject: "原主题", SearchText: "原文"}
	m, _, err := s.InsertMailMessageIfNew(p)
	if err != nil {
		t.Fatal(err)
	}
	_, inserted, err := s.InsertMailMessageIfNew(p)
	if err != nil || inserted {
		t.Fatalf("duplicate: %v %v", inserted, err)
	}
	stats, err := s.MailSearchTasksStats(context.Background(), user.ID)
	if err != nil || stats.Pending != 1 {
		t.Fatalf("duplicate created task: %+v %v", stats, err)
	}
	// 任务写入失败必须回滚邮件变更，而不是提交无法被索引发现的数据。
	if _, err = s.db.Exec(`DROP TABLE mail_search_tasks`); err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateMailMessageParsedContent(UpdateMailMessageContentParams{UserID: user.ID, ID: m.ID, Subject: "不应提交", SearchText: "不应提交"}); err == nil {
		t.Fatal("missing outbox must fail update")
	}
	after, err := s.FindMailMessageByUID(user.ID, account.ID, "INBOX", "1")
	if err != nil || after.Subject.String != "原主题" {
		t.Fatalf("update not atomic: %+v %v", after, err)
	}
	p.IMAPUID = "2"
	if _, _, err = s.InsertMailMessageIfNew(p); err == nil {
		t.Fatal("missing outbox must fail insert")
	}
	if _, err = s.FindMailMessageByUID(user.ID, account.ID, "INBOX", "2"); err != ErrNotFound {
		t.Fatalf("insert not rolled back: %v", err)
	}
	_ = s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.InsertMailMessageIfNew(p); err != nil {
		t.Fatalf("missing table migration: %v", err)
	}
}
