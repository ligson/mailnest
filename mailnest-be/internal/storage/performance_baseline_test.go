package storage

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 仅显式启用；默认临时 SQLite，MySQL 必须通过一次性测试库 DSN 提供。
// 不读取 config.yaml、不清数据库/OS 缓存、不接触真实用户原文。
func TestPerformanceBaseline(t *testing.T) {
	if os.Getenv("MAILNEST_RUN_PERF") != "1" {
		t.Skip("显式设置 MAILNEST_RUN_PERF=1 运行独立负载基线")
	}
	n := 10000
	if value, err := strconv.Atoi(os.Getenv("MAILNEST_PERF_MESSAGES")); err == nil && value >= 1000 && value <= 100000 {
		n = value
	}
	s := performanceStore(t)
	u, err := s.CreateUser(fmt.Sprintf("perf-%d", time.Now().UnixNano()), fmt.Sprintf("perf-%d@example.com", time.Now().UnixNano()), "synthetic-hash")
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.CreateMailAccount(MailAccount{UserID: u.ID, Email: "synthetic@example.com", DisplayName: "合成数据", IMAPHost: "example.com", IMAPPort: 993, IMAPUsername: "synthetic", IMAPPasswordEncoded: "synthetic", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	threads := make([]mailThreadModel, 100)
	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range threads {
		threads[i] = mailThreadModel{UserID: u.ID, AccountID: a.ID, Subject: fmt.Sprintf("会话-%d", i), NormalizedSubject: fmt.Sprintf("thread-%d", i), SortAt: &at, LastMessageAt: &at, MessageCount: n / 100}
	}
	if err = s.db.gormDB.CreateInBatches(&threads, 100).Error; err != nil {
		t.Fatal(err)
	}
	from, to, cc := "测试发件人 <sender@example.com>", "receiver@example.com", "cc@example.com"
	for start := 0; start < n; start += 200 {
		batch := make([]mailMessageModel, 0, 200)
		for i := start; i < min(start+200, n); i++ {
			subject := fmt.Sprintf("中文邮件 会议纪要 %d", i)
			body := strings.Repeat("合成正文数据 abc@example.com 123456。", 32)
			if i%197 == 0 {
				body += "稀疏命中 601"
			}
			sent := at.Add(time.Duration(i/4) * time.Second)
			threadID := threads[i%100].ID
			batch = append(batch, mailMessageModel{UserID: u.ID, AccountID: a.ID, ThreadID: &threadID, Folder: "INBOX", IMAPUID: strconv.Itoa(i), Subject: &subject, FromAddr: &from, ToAddrs: &to, CCAddrs: &cc, SearchText: &body, SortAt: &sent, ReceivedAt: &sent})
		}
		if err = s.db.gormDB.CreateInBatches(&batch, 200).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, thread := range threads {
		if err = s.RefreshMailThreadStats(u.ID, thread.ID); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("environment engine=%s messages=%d threads=100 go=%s cpus=%d samples=80 concurrency=8 cold=application-cache-bypass db-os-cache=not-cleared", s.db.dialect, n, runtime.Version(), runtime.NumCPU())
	queries := []struct {
		name     string
		messages ListMailMessagesQuery
		thread   bool
	}{
		{name: "list", messages: ListMailMessagesQuery{}},
		{name: "search-sparse", messages: ListMailMessagesQuery{Body: "601"}},
		{name: "search-wide", messages: ListMailMessagesQuery{Keyword: "中文"}},
		{name: "search-one-character", messages: ListMailMessagesQuery{Keyword: "中"}},
		{name: "search-sender", messages: ListMailMessagesQuery{From: "sender@example.com"}},
		{name: "threads", thread: true},
	}
	for _, test := range queries {
		q := test.messages
		q.UserID = u.ID
		q.SummaryOnly = true
		q.SkipTotal = true
		q.Limit = 20
		read := func(cached bool) error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			finish := s.Workload().Interactive()
			defer finish()
			if test.thread {
				tq := ListMailThreadsQuery{UserID: u.ID, Limit: 20, SkipTotal: true}
				if cached {
					_, _, err := s.ListMailThreadsContext(ctx, tq)
					return err
				}
				_, _, err := s.listMailThreadsUncached(ctx, tq)
				return err
			}
			if cached {
				_, _, err := s.ListMailMessagesByQueryContext(ctx, q)
				return err
			}
			_, _, err := s.listMailMessagesUncached(ctx, q)
			return err
		}
		for _, phase := range []string{"cache-bypass", "warm-cache", "concurrent-bypass", "background-writes"} {
			concurrency := 1
			if phase == "concurrent-bypass" || phase == "background-writes" {
				concurrency = 8
			}
			ctx, cancel := context.WithCancel(context.Background())
			var writer sync.WaitGroup
			var writeErrors, writes atomic.Int64
			firstWrite := make(chan struct{})
			if phase == "background-writes" {
				writer.Add(1)
				go func() {
					defer writer.Done()
					for ctx.Err() == nil {
						release, e := s.Workload().Background(ctx, "persist")
						if e != nil {
							return
						}
						_, e = s.db.Exec("UPDATE mail_messages SET updated_at=? WHERE user_id=? AND account_id=? AND imap_uid=?", time.Now(), u.ID, a.ID, "0")
						release()
						if e != nil {
							writeErrors.Add(1)
						}
						if writes.Add(1) == 1 {
							close(firstWrite)
						}
						select {
						case <-ctx.Done():
							return
						case <-time.After(10 * time.Millisecond):
						}
					}
				}()
				select {
				case <-firstWrite:
				case <-time.After(5 * time.Second):
					cancel()
					writer.Wait()
					t.Fatal("后台写入未启动")
				}
			}
			if e := read(phase == "warm-cache"); e != nil {
				t.Fatal(e)
			}
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			durations := make([]time.Duration, 80)
			var next, failures atomic.Int64
			var workers sync.WaitGroup
			for i := 0; i < concurrency; i++ {
				workers.Add(1)
				go func() {
					defer workers.Done()
					for {
						index := int(next.Add(1)) - 1
						if index >= len(durations) {
							return
						}
						start := time.Now()
						if e := read(phase == "warm-cache"); e != nil {
							failures.Add(1)
						}
						durations[index] = time.Since(start)
						if phase == "background-writes" {
							time.Sleep(40 * time.Millisecond)
						}
					}
				}()
			}
			workers.Wait()
			runtime.ReadMemStats(&after)
			cancel()
			writer.Wait()
			sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
			t.Logf("case=%s phase=%s p50=%s p95=%s max=%s errors=%d writes=%d writeErrors=%d aggregate_alloc_bytes=%d", test.name, phase, durations[39], durations[75], durations[79], failures.Load(), writes.Load(), writeErrors.Load(), after.TotalAlloc-before.TotalAlloc)
			if failures.Load() != 0 || writeErrors.Load() != 0 {
				t.Fail()
			}
		}
	}
	q := ListMailMessagesQuery{UserID: u.ID, CountOnly: true}
	_, total, err := s.ListMailMessagesByQuery(q)
	if err != nil || total != n {
		t.Fatalf("负载后邮件数不一致: got=%d want=%d err=%v", total, n, err)
	}
	var rawTotal int
	if err = s.db.QueryRow("SELECT COUNT(*) FROM mail_messages WHERE user_id=?", u.ID).Scan(&rawTotal); err != nil || rawTotal != n {
		t.Fatal("原始记录数不一致", rawTotal, err)
	}
}
