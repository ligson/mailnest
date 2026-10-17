package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestReadCacheSingleFlightIndependentCancellation(t *testing.T) {
	var cache readCache
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	loader := func(ctx context.Context) (any, error) {
		calls.Add(1)
		close(started)
		select {
		case <-release:
			return 42, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { _, err := cache.get(ctx, "same", loader); first <- err }()
	<-started
	second := make(chan error, 1)
	go func() {
		v, err := cache.get(context.Background(), "same", loader)
		if err == nil && v != 42 {
			err = fmt.Errorf("错误值 %v", v)
		}
		second <- err
	}()
	deadline := time.Now().Add(time.Second)
	for {
		cache.mu.Lock()
		refs := cache.flights["same"].refs
		cache.mu.Unlock()
		if refs == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("等待者未加入")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("取消失败 %v", err)
	}
	close(release)
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if _, err := cache.get(context.Background(), "same", loader); err != nil || calls.Load() != 1 {
		t.Fatalf("没有合并或缓存 %d %v", calls.Load(), err)
	}
	cache.mu.Lock()
	entry := cache.entries["same"]
	entry.expires = time.Now().Add(-time.Second)
	cache.entries["same"] = entry
	cache.mu.Unlock()
	if v, err := cache.get(context.Background(), "same", func(context.Context) (any, error) { return 43, nil }); err != nil || v != 43 {
		t.Fatal("过期缓存未刷新")
	}
}

func TestReadCacheAllCanceledErrorsAndBounds(t *testing.T) {
	var cache readCache
	ctx, cancel := context.WithCancel(context.Background())
	started, stopped := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := cache.get(ctx, "cancel", func(work context.Context) (any, error) {
			close(started)
			<-work.Done()
			close(stopped)
			return nil, work.Err()
		})
		done <- err
	}()
	<-started
	cancel()
	<-done
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("无人等待后查询未取消")
	}
	var calls int
	for i := 0; i < 2; i++ {
		_, _ = cache.get(context.Background(), "error", func(context.Context) (any, error) { calls++; return nil, errors.New("失败") })
	}
	if calls != 2 {
		t.Fatal("错误被缓存")
	}
	for i := 0; i < 200; i++ {
		_, err := cache.get(context.Background(), fmt.Sprint(i), func(context.Context) (any, error) { return i, nil })
		if err != nil {
			t.Fatal(err)
		}
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if len(cache.entries) > 128 || len(cache.flights) != 0 {
		t.Fatalf("缓存越界 %d/%d", len(cache.entries), len(cache.flights))
	}
}

func performanceStore(t *testing.T) *Store {
	t.Helper()
	var s *Store
	var err error
	if dsn := os.Getenv("MAILNEST_TEST_MYSQL_DSN"); dsn != "" {
		s, err = OpenWithOptions(DatabaseOptions{Driver: "mysql", DSN: dsn})
	} else {
		s, err = Open(filepath.Join(t.TempDir(), "read.db"))
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestMailboxReadDeferredCountMigrationAndIsolation(t *testing.T) {
	s := performanceStore(t)
	userID := checkMailSearchResults(t, s)
	q := ListMailMessagesQuery{UserID: userID, SystemFolder: "inbox", SummaryOnly: true, Limit: 1, SkipTotal: true}
	page, total, err := s.ListMailMessagesByQuery(q)
	if err != nil || total != -1 || len(page) != 2 {
		t.Fatalf("延迟统计失败 %d %d %v", len(page), total, err)
	}
	firstID := page[0].ID
	page[0].Subject.String = "不能污染缓存"
	page, _, err = s.ListMailMessagesByQuery(q)
	if err != nil || page[0].Subject.String == "不能污染缓存" {
		t.Fatal("缓存可被调用者修改")
	}
	q.CountOnly = true
	countItems, total, err := s.ListMailMessagesByQuery(q)
	if err != nil || total != 2 || len(countItems) != 0 {
		t.Fatalf("精确统计失败 %d %v", total, err)
	}
	q.UserID = -1
	_, total, err = s.ListMailMessagesByQuery(q)
	if err != nil || total != 0 {
		t.Fatal("跨用户读取")
	}
	revision := s.MailReadVersion()
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("UPDATE mail_messages SET subject=? WHERE id=?", "回滚", firstID); err != nil {
		t.Fatal(err)
	}
	if s.MailReadVersion() != revision {
		t.Fatal("未提交即失效")
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if s.MailReadVersion() != revision {
		t.Fatal("回滚影响缓存版本")
	}
	tx, err = s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("UPDATE mail_messages SET subject=? WHERE id=?", "已提交", firstID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if s.MailReadVersion() == revision {
		t.Fatal("提交后未失效")
	}
	q = ListMailMessagesQuery{UserID: userID, SystemFolder: "inbox", SummaryOnly: true, Limit: 1, SkipTotal: true}
	page, _, err = s.ListMailMessagesByQuery(q)
	if err != nil || page[0].Subject.String != "已提交" {
		t.Fatal("提交后返回旧缓存")
	}
	// 模拟旧版本回退期间没有写新增派生列；重新升级必须可重复修复。
	old := time.Date(2025, 2, 3, 4, 5, 6, 0, time.UTC)
	if _, err = s.db.Exec("UPDATE mail_messages SET sent_at=?,sort_at=NULL WHERE id=?", old, firstID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = s.ensureMailSortTime(); err != nil {
			t.Fatal(err)
		}
	}
	var sorted, sent sql.NullTime
	if err = s.db.QueryRow("SELECT sort_at,sent_at FROM mail_messages WHERE id=?", firstID).Scan(&sorted, &sent); err != nil {
		t.Fatal(err)
	}
	if !sorted.Time.Equal(old) || !sent.Time.Equal(old) {
		t.Fatalf("迁移改变原日期或排序错误 %v/%v", sorted, sent)
	}
	if s.db.dialect == dialectSQLite {
		s.db.SetMaxOpenConns(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	threads, _, err := s.ListMailThreadsContext(ctx, ListMailThreadsQuery{UserID: userID, SystemFolder: "inbox", Limit: 1, SkipTotal: true})
	if err != nil || len(threads) != 1 {
		t.Fatalf("批量会话/单连接失败 %v", err)
	}
	latest := threads[0].LatestMessage
	if latest.ID == firstID || latest.SearchText.Valid || latest.RawPath.Valid || latest.CCAddrs.String != "cc@example.com" {
		t.Fatalf("会话摘要内容错误 %+v", latest)
	}
	threads[0].Participants[0] = "不能污染缓存"
	threads, _, err = s.ListMailThreadsContext(ctx, ListMailThreadsQuery{UserID: userID, SystemFolder: "inbox", Limit: 1, SkipTotal: true})
	if err != nil || threads[0].Participants[0] == "不能污染缓存" {
		t.Fatal("会话参与者污染缓存")
	}
	_, n, err := s.ListMailThreadsContext(ctx, ListMailThreadsQuery{UserID: userID, SystemFolder: "inbox", CountOnly: true})
	if err != nil || n != 1 {
		t.Fatalf("会话统计 %d %v", n, err)
	}
}

func TestContactResolutionGlobalAmbiguityAndIsolation(t *testing.T) {
	s := performanceStore(t)
	suffix := fmt.Sprint(time.Now().UnixNano())
	u, err := s.CreateUser("contact"+suffix, "contact"+suffix+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	for _, email := range []string{"zhang@one.com", "zhang@two.com", "unique@one.com", "a_b@one.com", "axb@two.com"} {
		if _, err = s.CreateContact(CreateContactParams{UserID: u.ID, Email: email, Nickname: "昵称"}); err != nil {
			t.Fatal(err)
		}
	}
	result, err := s.ResolveContacts(context.Background(), u.ID, []string{"zhang@one.com", "unique", "a_b"})
	if err != nil || len(result.Items) != 3 || len(result.AmbiguousPrefixes) != 1 || result.AmbiguousPrefixes[0] != "zhang" {
		t.Fatalf("解析错误 %+v %v", result, err)
	}
	result.Items[0].Email = "污染"
	result.AmbiguousPrefixes[0] = "污染"
	result, err = s.ResolveContacts(context.Background(), u.ID, []string{"zhang@one.com", "unique", "a_b"})
	if err != nil || result.Items[0].Email == "污染" || result.AmbiguousPrefixes[0] != "zhang" {
		t.Fatal("缓存污染")
	}
	other, err := s.ResolveContacts(context.Background(), -1, []string{"unique"})
	if err != nil || len(other.Items) != 0 {
		t.Fatal("跨用户联系人")
	}
	if _, err = s.CreateContact(CreateContactParams{UserID: u.ID, Email: "unique@two.com"}); err != nil {
		t.Fatal(err)
	}
	result, err = s.ResolveContacts(context.Background(), u.ID, []string{"unique"})
	if err != nil || len(result.Items) != 0 || len(result.AmbiguousPrefixes) != 1 {
		t.Fatal("通讯录修改未使缓存失效")
	}
}
