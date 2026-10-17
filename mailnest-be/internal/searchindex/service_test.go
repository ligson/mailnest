package searchindex

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

	"mailnest-be/internal/config"
	"mailnest-be/internal/storage"
)

type fixture struct {
	store                                      *storage.Store
	user, other, account, otherAccount, thread int64
}

func newFixture(t *testing.T, mysql bool) *fixture {
	t.Helper()
	var store *storage.Store
	var err error
	if mysql {
		dsn := os.Getenv("MAILNEST_TEST_MYSQL_DSN")
		if dsn == "" {
			t.Skip("未配置一次性 MySQL 测试库")
		}
		store, err = storage.OpenWithOptions(storage.DatabaseOptions{Driver: "mysql", DSN: dsn})
	} else {
		store, err = storage.Open(filepath.Join(t.TempDir(), "mail.db"))
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	suffix := fmt.Sprint(time.Now().UnixNano())
	u, err := store.CreateUser("s"+suffix, "s"+suffix+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateUser("o"+suffix, "o"+suffix+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	account, err := store.CreateMailAccount(storage.MailAccount{UserID: u.ID, DisplayName: "搜索测试", Email: "sender@example.com", IMAPHost: "example.com", IMAPPort: 993, IMAPUsername: "sender", IMAPPasswordEncoded: "encrypted", PollIntervalMinutes: 10, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	otherAccount, err := store.CreateMailAccount(storage.MailAccount{UserID: other.ID, DisplayName: "隔离测试", Email: "other@example.com", IMAPHost: "example.com", IMAPPort: 993, IMAPUsername: "other", IMAPPasswordEncoded: "encrypted", PollIntervalMinutes: 10, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	thread, err := store.CreateMailThread(storage.CreateMailThreadParams{UserID: u.ID, AccountID: account.ID, Subject: "搜索"})
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{store: store, user: u.ID, other: other.ID, account: account.ID, otherAccount: otherAccount.ID, thread: thread.ID}
}
func (f *fixture) add(t *testing.T, subject, body string, foreign bool) storage.MailMessage {
	t.Helper()
	user, account := f.user, f.account
	thread := sql.NullInt64{Int64: f.thread, Valid: true}
	if foreign {
		user, account = f.other, f.otherAccount
		thread.Valid = false
	}
	m, inserted, err := f.store.InsertMailMessageIfNew(storage.CreateMailMessageParams{UserID: user, AccountID: account, ThreadID: thread, Folder: "INBOX", IMAPUID: fmt.Sprint(time.Now().UnixNano()), Subject: subject, FromAddr: "张海涛 <ABC@example.com>", ToAddrs: "receiver@example.com", CCAddrs: "copy@example.com", SearchText: body, ReceivedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true}})
	if err != nil || !inserted {
		t.Fatalf("insert: %v %v", inserted, err)
	}
	return m
}
func openService(t *testing.T, f *fixture, path, mode string) *Service {
	t.Helper()
	s, err := Open(f.store, config.SearchConfig{Mode: mode, BatchSize: 2, BatchDelayMS: 10, MaxCandidates: 20000}, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func pump(t *testing.T, s *Service) {
	t.Helper()
	for i := 0; i < 20000; i++ {
		_, err := s.step(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		s.mu.RLock()
		done := s.active != nil && s.building == nil && !s.requested
		s.mu.RUnlock()
		stats, err := s.store.MailSearchTasksStats(context.Background(), 0)
		if err != nil {
			t.Fatal(err)
		}
		if done && stats.Pending == 0 {
			return
		}
	}
	t.Fatal("索引没有完成")
}
func assertMatches(t *testing.T, f *fixture, s *Service, q storage.ListMailMessagesQuery) {
	t.Helper()
	q.UserID = f.user
	q.SummaryOnly = true
	f.store.SetMailSearchProvider(nil)
	expected, total, err := f.store.ListMailMessagesByQuery(q)
	if err != nil {
		t.Fatal(err)
	}
	f.store.SetMailSearchProvider(s)
	actual, count, err := f.store.ListMailMessagesByQuery(q)
	if err != nil {
		t.Fatal(err)
	}
	ids := func(messages []storage.MailMessage) []int64 {
		result := make([]int64, len(messages))
		for i, m := range messages {
			if m.UserID != f.user {
				t.Fatal("跨用户搜索结果")
			}
			result[i] = m.ID
		}
		return result
	}
	if total != count || !reflect.DeepEqual(ids(expected), ids(actual)) {
		t.Fatalf("query %+v: count %d/%d ids %v/%v", q, count, total, ids(actual), ids(expected))
	}
	tq := storage.ListMailThreadsQuery{UserID: f.user, Keyword: q.Keyword, From: q.From, Subject: q.Subject, Body: q.Body, Limit: q.Limit, Offset: q.Offset, SystemFolder: q.SystemFolder}
	f.store.SetMailSearchProvider(nil)
	before, beforeTotal, err := f.store.ListMailThreads(tq)
	if err != nil {
		t.Fatal(err)
	}
	f.store.SetMailSearchProvider(s)
	after, afterTotal, err := f.store.ListMailThreads(tq)
	if err != nil {
		t.Fatal(err)
	}
	if beforeTotal != afterTotal || !reflect.DeepEqual(before, after) {
		t.Fatalf("thread query mismatch %+v", tq)
	}
}
func checkSearch(t *testing.T, mysql bool) {
	f := newFixture(t, mysql)
	for _, text := range []string{"601 中文测试 张海涛 abc@example.com abcdef 60%01", "606 会议安排 60--01", "école Straße ＡＢＣ KBC ångström a\u200bb", "𠮷野家 かな 가나 👍"} {
		f.add(t, text, text, false)
	}
	f.add(t, "foreign-only 601", "跨用户不可见", true)
	// 模拟升级前的历史邮件：无 outbox 也必须完整回填。
	tasks, err := f.store.ListMailSearchTasks(context.Background(), 200)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.store.CompleteMailSearchTasks(context.Background(), tasks); err != nil {
		t.Fatal(err)
	}
	s := openService(t, f, t.TempDir(), "active")
	pump(t, s)
	for _, keyword := range []string{"601", "中文测试", "张海涛", "中", "a", "an", "@example.com", "example.com", "ABC", "60%01", "60_01", `60\%01`, "+601 -606", "不存在", "école", "ecole", "strasse", "kbc", "ab", "吉野", "foreign-only", "%", "_"} {
		assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: keyword})
		assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: keyword, SkipTotal: true})
	}
	for _, q := range []storage.ListMailMessagesQuery{{From: "海涛"}, {From: "abc@example.com"}, {Subject: "606"}, {Body: "601"}, {Keyword: "601", Subject: "606"}, {Keyword: "601", From: "海涛"}, {Keyword: "中文", Limit: 1, Offset: 1}, {Keyword: "601", SystemFolder: "sent"}} {
		assertMatches(t, f, s, q)
	}
	// 新增、修复更新未处理时仍可通过 pending overlay 搜到，旧正文候选经 SQL 复核被排除。
	added := f.add(t, "新增唯一邮件", "旧正文 13579", false)
	assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: "13579"})
	pump(t, s)
	if err = f.store.UpdateMailMessageParsedContent(storage.UpdateMailMessageContentParams{UserID: f.user, ID: added.ID, Subject: "修复后主题", SearchText: "新正文 24680"}); err != nil {
		t.Fatal(err)
	}
	assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: "24680"})
	assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: "24680", SkipTotal: true})
	assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: "13579"})
	pump(t, s)
	assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: "24680"})
	if err = f.store.MarkMailMessageDeleted(f.user, added.ID, true); err != nil {
		t.Fatal(err)
	}
	assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: "24680"})
	assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: "24680", OnlyDeleted: true})
	status, err := s.Status(context.Background(), f.other, false)
	if err != nil {
		t.Fatal(err)
	}
	if status.Total != 1 || status.ShadowChecks != nil {
		t.Fatalf("status isolation: %+v", status)
	}
	// 溢出不能截断结果，必须回退。
	s.cfg.MaxCandidates = 1
	c, err := s.Candidates(context.Background(), storage.MailSearchTextQuery{UserID: f.user, Keyword: "a"})
	if err != nil || c.Available {
		t.Fatalf("overflow must fall back: %+v %v", c, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.Candidates(ctx, storage.MailSearchTextQuery{UserID: f.user, Keyword: "601"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}
func TestBleveSearchSQLite(t *testing.T) { checkSearch(t, false) }
func TestBleveSearchMySQL(t *testing.T)  { checkSearch(t, true) }

func TestRebuildResumeAndLock(t *testing.T) {
	f := newFixture(t, false)
	for i := 0; i < 9; i++ {
		f.add(t, fmt.Sprint("历史邮件", i), "历史全文", false)
	}
	path := t.TempDir()
	s := openService(t, f, path, "active")
	if _, err := Open(f.store, config.SearchConfig{}, path); err == nil {
		t.Fatal("must reject second writer")
	}
	if _, err := s.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	cp := s.progress
	if cp.Cursor == 0 {
		t.Fatal("checkpoint not saved")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openService(t, f, path, "active")
	if s.progress != cp {
		t.Fatal("checkpoint not restored")
	}
	pump(t, s)
	first := s.manifest.Active
	if err := s.RequestRebuild(); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(s.RequestRebuild(), ErrBusy) {
		t.Fatal("parallel rebuild must conflict")
	}
	f.add(t, "重建期间新信", "全量之后的新邮件", false)
	if _, err := s.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.active == nil || s.manifest.Active != first {
		t.Fatal("old index must remain serving during build")
	}
	assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: "新邮件"})
	pump(t, s)
	if s.manifest.Active == first || s.manifest.Previous != first {
		t.Fatal("generation not published")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openService(t, f, path, "active")
	assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: "全量"})
	// 损坏的 active 保留原目录，服务自动回退并建立新代际，不修改邮件。
	corrupt := s.manifest.Active
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "search", corrupt, "index_meta.json"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	s = openService(t, f, path, "active")
	if s.active != nil {
		t.Fatal("corrupt active should not be served")
	}
	assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: "全量"})
	pump(t, s)
	if _, err := os.Stat(filepath.Join(path, "search", corrupt)); err != nil {
		t.Fatal("corrupt files must be retained")
	}
}

func TestShadowAndRetry(t *testing.T) {
	f := newFixture(t, false)
	f.add(t, "影子测试", strings.Repeat("中文测试601", 10000), false)
	s := openService(t, f, t.TempDir(), "shadow")
	pump(t, s)
	assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: "601"})
	if s.shadowChecks.Load() == 0 || s.shadowMissing.Load() != 0 {
		t.Fatal("shadow sample not observed")
	}
	added := f.add(t, "重试邮件", "失败保留", false)
	tasks, err := f.store.ListMailSearchTasks(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.store.RetryMailSearchTasks(context.Background(), tasks); err != nil {
		t.Fatal(err)
	}
	stats, err := f.store.MailSearchTasksStats(context.Background(), f.user)
	if err != nil || stats.Pending != 1 || stats.Failed != 1 {
		t.Fatalf("retry stats: %+v %v", stats, err)
	}
	due, err := f.store.ListMailSearchTasks(context.Background(), 100)
	if err != nil || len(due) != 0 {
		t.Fatalf("retry delay: %v %v", due, err)
	}
	assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: "失败保留"})
	// 精确确认已读事件，不得删除读取后新增的更新事件。
	if err = f.store.UpdateMailMessageParsedContent(storage.UpdateMailMessageContentParams{UserID: f.user, ID: added.ID, Subject: "第二个事件", SearchText: "最新版本"}); err != nil {
		t.Fatal(err)
	}
	if err = f.store.CompleteMailSearchTasks(context.Background(), tasks); err != nil {
		t.Fatal(err)
	}
	stats, _ = f.store.MailSearchTasksStats(context.Background(), f.user)
	if stats.Pending != 1 {
		t.Fatalf("new event was lost: %+v", stats)
	}
	pump(t, s)
	assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: "最新版本"})
	s.recordError(errors.New("已恢复的历史失败"))
	recovered, err := s.Status(context.Background(), f.user, false)
	if err != nil || recovered.Message != "" {
		t.Fatalf("recovered status: %+v %v", recovered, err)
	}
	tokens, _ := grams(strings.Repeat("中文测试601", 10000))
	if len(tokens) > 20 {
		t.Fatal("repeated body must be deduplicated")
	}
}

func TestBackgroundCloseAndConcurrentSearch(t *testing.T) {
	f := newFixture(t, false)
	for i := 0; i < 25; i++ {
		f.add(t, fmt.Sprint("并发", i), "中文601", false)
	}
	s := openService(t, f, t.TempDir(), "active")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	deadline := time.Now().Add(10 * time.Second)
	for {
		status, err := s.Status(context.Background(), f.user, false)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.Candidates(context.Background(), storage.MailSearchTextQuery{UserID: f.user, Keyword: "601"})
		if err != nil {
			t.Fatal(err)
		}
		if status.Available && status.Phase == "ready" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker timeout")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openService(t, f, s.path[:len(s.path)-len("/search")], "active")
	if reopened.active == nil {
		t.Fatal("close lost active index")
	}
}
