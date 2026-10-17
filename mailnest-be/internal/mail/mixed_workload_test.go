package mail

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mailnest-be/internal/config"
	"mailnest-be/internal/searchindex"
	"mailnest-be/internal/storage"
)

// 显式运行的小型端到端负载：真实 EML 解析/文件写入、模拟 IMAP、Bleve worker 与前台读取。
// 远端收信使用 FakeFetcher，不连接真实邮箱，不读取生产配置。
func TestMixedImportSyncIndexReads(t *testing.T) {
	if os.Getenv("MAILNEST_RUN_PERF") != "1" {
		t.Skip("显式开启独立混合负载")
	}
	dir := t.TempDir()
	store, err := storage.Open(filepath.Join(dir, "mixed.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	u, err := store.CreateUser("mixed", "mixed@example.com", "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	a, err := store.CreateMailAccount(storage.MailAccount{UserID: u.ID, DisplayName: "混合负载", Email: "mixed@example.com", IMAPHost: "example.com", IMAPPort: 993, IMAPUsername: "mixed", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	fetcher := &FakeFetcher{}
	svc := NewService(store, fetcher, nil, dir, "synthetic-secret")
	raw := func(i int) []byte {
		return []byte(fmt.Sprintf("Message-ID: <mixed-%d@example.com>\r\nFrom: sender@example.com\r\nTo: mixed@example.com\r\nSubject: mixed 601 %d\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s", i, i, strings.Repeat("混合负载正文 601 ", 512)))
	}
	for i := 0; i < 50; i++ {
		if _, err = svc.ImportEML(u.ID, a.ID, "INBOX", raw(i)); err != nil {
			t.Fatal(err)
		}
	}
	for i := 100; i < 150; i++ {
		m := fetchedMessageFromRaw(raw(i))
		m.UID = fmt.Sprint(i)
		fetcher.Messages = append(fetcher.Messages, m)
	}
	index, err := searchindex.Open(store, config.SearchConfig{Mode: "active", BatchSize: 2, BatchDelayMS: 10, MaxCandidates: 2000}, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	store.SetMailSearchProvider(index)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	index.Start(ctx)
	// 等首代索引可用，确保前台实际走 Bleve 流式检索，而不是只验证建索引期间的 SQL 回退。
	for {
		status, e := index.Status(ctx, u.ID, false)
		if e != nil {
			t.Fatal(e)
		}
		if status.Available && status.Phase == "ready" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("初始索引未就绪")
		case <-time.After(20 * time.Millisecond):
		}
	}
	var writers sync.WaitGroup
	if err = index.RequestRebuild(); err != nil {
		t.Fatal(err)
	}
	var failures atomic.Int64
	writers.Add(2)
	go func() {
		defer writers.Done()
		for i := 50; i < 100; i++ {
			if _, e := svc.ImportEMLContext(ctx, u.ID, a.ID, "INBOX", raw(i)); e != nil {
				failures.Add(1)
				return
			}
		}
	}()
	go func() {
		defer writers.Done()
		if _, e := svc.SyncInbox(u.ID, a.ID); e != nil {
			failures.Add(1)
		}
	}()
	durations := make([]time.Duration, 160)
	var next atomic.Int64
	var readers sync.WaitGroup
	for i := 0; i < 4; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				n := int(next.Add(1)) - 1
				if n >= len(durations) {
					return
				}
				start := time.Now()
				finish := store.Workload().Interactive()
				messages, _, e := store.ListMailMessagesByQueryContext(ctx, storage.ListMailMessagesQuery{UserID: u.ID, Keyword: "601", SummaryOnly: true, SkipTotal: true, Limit: 20})
				if e != nil || len(messages) == 0 {
					failures.Add(1)
				}
				if e == nil && len(messages) > 0 {
					if _, e = store.FindMailMessageByID(u.ID, messages[0].ID); e != nil {
						failures.Add(1)
					}
				}
				finish()
				durations[n] = time.Since(start)
				if n%8 == 0 {
					if _, _, e := store.DeferredMessageCount(ctx, storage.ListMailMessagesQuery{UserID: u.ID, Keyword: "601", CountOnly: true}); e != nil {
						failures.Add(1)
					}
				}
				time.Sleep(20 * time.Millisecond)
			}
		}()
	}
	readers.Wait()
	writers.Wait()
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	_, total, err := store.ListMailMessagesByQuery(storage.ListMailMessagesQuery{UserID: u.ID, CountOnly: true})
	if err != nil || total != 150 || failures.Load() != 0 {
		t.Fatalf("混合负载失败 total=%d errors=%d err=%v", total, failures.Load(), err)
	}
	for {
		stats, e := store.MailSearchTasksStats(ctx, u.ID)
		if e != nil {
			t.Fatal(e)
		}
		if stats.Pending == 0 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("索引未排空")
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Logf("mixed sqlite samples=160 concurrency=4 seed=50 imported=50 synced=50 index=active count=deferred p50=%s p95=%s max=%s errors=%d", durations[79], durations[151], durations[159], failures.Load())
}
