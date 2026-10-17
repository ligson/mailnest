package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMailSearchTerms(t *testing.T) {
	for _, tc := range []struct{ input, expected string }{
		{"601", `+"60" +"01"`},
		{"张海涛", `+"张海" +"海涛"`},
		{"ABC@EXAMPLE.COM", `+"bc" +"ex" +"mp" +"pl" +"le" +"co" +"om"`},
		{"a", ""}, {"an", ""}, {"中", ""}, {"%_+@", ""},
		{`ab\%601`, ""}, {"60%01", `+"60" +"01"`},
		{"606060", `+"60" +"06"`},
		{"KBC", `+"bc"`},
	} {
		if got := mailSearchTerms(tc.input, []string{"a", "i", "an"}); got != tc.expected {
			t.Errorf("%q: got %q want %q", tc.input, got, tc.expected)
		}
	}
	if len(strings.Fields(mailSearchTerms(strings.Repeat("0123456789", 100), nil))) != 8 {
		t.Fatal("must bound candidate terms")
	}
}

func TestMailSearchWhereKeepsExactFieldChecks(t *testing.T) {
	s := &Store{mailSearch: mailSearchCapabilities{ngram: true}}
	where, args := s.mailSearchWhere("601", "张海涛", "主题", "正文")
	if strings.Count(where, "MATCH(") != 4 || strings.Count(where, "LIKE ?") != 8 || len(args) != 12 {
		t.Fatalf("unexpected query: %s %#v", where, args)
	}
	where, args = (&Store{}).mailSearchWhere("601", "", "", "")
	if strings.Contains(where, "MATCH") || len(args) != 5 {
		t.Fatal("fallback must preserve LIKE")
	}
}

func TestMailSearchSQLiteFallbackAndCancellation(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	checkMailSearchResults(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := s.ListMailMessagesByQueryContext(ctx, ListMailMessagesQuery{UserID: 1, Keyword: "601"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if _, _, err := s.ListMailThreadsContext(ctx, ListMailThreadsQuery{UserID: 1, Keyword: "601"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected thread cancellation, got %v", err)
	}
}

// 显式提供一次性测试库，不读取生产配置；本机 Docker MySQL 集成回归入口。
func TestMailSearchMySQLNgram(t *testing.T) {
	dsn := os.Getenv("MAILNEST_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置一次性 MySQL 测试库")
	}
	s, err := OpenWithOptions(DatabaseOptions{Driver: "mysql", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// 先写入兼容模式数据，建索引后验证历史邮件也进入全文索引。
	legacyUserID := checkMailSearchResults(t, s)
	if err := s.PrepareMailSearchIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, legacyTotal, err := s.ListMailMessagesByQuery(ListMailMessagesQuery{UserID: legacyUserID, Body: "987654", SummaryOnly: true})
	if err != nil || legacyTotal != 1 {
		t.Fatalf("historical message indexing: total=%d err=%v", legacyTotal, err)
	}
	if err := s.PrepareMailSearchIndex(context.Background()); err != nil {
		t.Fatalf("idempotency: %v", err)
	}
	if !s.mailSearch.ngram {
		t.Fatal("ngram not enabled")
	}
	reopened, err := OpenWithOptions(DatabaseOptions{Driver: "mysql", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	if !reopened.mailSearch.ngram {
		t.Fatal("persisted index was not detected after reopen")
	}
	reopened.Close()
	checkMailSearchResults(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	var slept int
	err = s.db.QueryRowContext(ctx, `SELECT SLEEP(2)`).Scan(&slept)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > time.Second {
		t.Fatalf("in-flight mysql query was not canceled promptly: %v", err)
	}
}

func checkMailSearchResults(t *testing.T, s *Store) int64 {
	t.Helper()
	suffix := fmt.Sprint(time.Now().UnixNano())
	user, err := s.CreateUser("search"+suffix, "search"+suffix+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := s.CreateUser("other"+suffix, "other"+suffix+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	account, err := s.CreateMailAccount(MailAccount{UserID: user.ID, DisplayName: "搜索测试", Email: "test@example.com", IMAPHost: "example.com", IMAPPort: 993, IMAPUsername: "test", IMAPPasswordEncoded: "encrypted", PollIntervalMinutes: 10, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	thread, err := s.CreateMailThread(CreateMailThreadParams{UserID: user.ID, AccountID: account.ID, Subject: "搜索"})
	if err != nil {
		t.Fatal(err)
	}
	for i, text := range []string{"601 中文测试 张海涛 abc@example.com abcdef 60%01", "606 会议 安排 another@example.com 60--01", "601 中文测试 不应跨用户出现"} {
		owner := user.ID
		if i == 2 {
			owner = foreign.ID
		}
		_, _, err := s.InsertMailMessageIfNew(CreateMailMessageParams{UserID: owner, AccountID: account.ID, ThreadID: sql.NullInt64{Int64: thread.ID, Valid: true}, Folder: "INBOX", IMAPUID: fmt.Sprint(i), Subject: text, FromAddr: "张海涛 <abc@example.com>", ToAddrs: "receiver@example.com", CCAddrs: "cc@example.com", SearchText: text, ReceivedAt: sql.NullTime{Time: time.Now(), Valid: true}})
		if err != nil {
			t.Fatal(err)
		}
	}
	queries := []ListMailMessagesQuery{
		{Keyword: "601"}, {Keyword: "中文测试"}, {Keyword: "张海涛"}, {Keyword: "example.com"},
		{Keyword: "abc"}, {Keyword: "ABC"}, {Keyword: "中文"}, {Keyword: "中"}, {Keyword: "a"}, {Keyword: "an"},
		{Keyword: "60%01"}, {Keyword: "60_01"}, {Keyword: `60\%01`}, {Keyword: "+601 -606"}, {Keyword: "不存在"},
		{From: "海涛"}, {From: "abc@example.com"}, {Subject: "606"}, {Body: "601"},
		{Keyword: "601", Subject: "606"}, {Keyword: "601", From: "海涛"},
	}
	capabilities := s.mailSearch
	for _, q := range queries {
		q.UserID = user.ID
		q.SummaryOnly = true
		q.SystemFolder = "inbox"
		s.mailSearch = mailSearchCapabilities{}
		expected, total, err := s.ListMailMessagesByQuery(q)
		if err != nil {
			t.Fatal(err)
		}
		s.mailSearch = capabilities
		actual, gotTotal, err := s.ListMailMessagesByQuery(q)
		if err != nil {
			t.Fatal(err)
		}
		if gotTotal != total || !reflect.DeepEqual(actual, expected) {
			t.Fatalf("search mismatch %+v: got %d want %d", q, gotTotal, total)
		}
		for _, m := range actual {
			if m.UserID != user.ID {
				t.Fatal("cross-user result")
			}
		}
		tq := ListMailThreadsQuery{UserID: user.ID, Keyword: q.Keyword, From: q.From, Subject: q.Subject, Body: q.Body, SystemFolder: "inbox"}
		s.mailSearch = mailSearchCapabilities{}
		before, beforeTotal, err := s.ListMailThreads(tq)
		if err != nil {
			t.Fatal(err)
		}
		s.mailSearch = capabilities
		after, afterTotal, err := s.ListMailThreads(tq)
		if err != nil {
			t.Fatal(err)
		}
		if beforeTotal != afterTotal || !reflect.DeepEqual(before, after) {
			t.Fatalf("thread mismatch %+v", tq)
		}
	}
	if _, err := s.db.Exec(`UPDATE mail_messages SET search_text=? WHERE user_id=? AND imap_uid='0'`, "更新后可搜索 987654", user.ID); err != nil {
		t.Fatal(err)
	}
	_, total, err := s.ListMailMessagesByQuery(ListMailMessagesQuery{UserID: user.ID, Body: "987654", SummaryOnly: true})
	if err != nil || total != 1 {
		t.Fatalf("index update: total=%d err=%v", total, err)
	}
	return user.ID
}
