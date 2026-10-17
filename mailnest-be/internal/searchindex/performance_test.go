package searchindex

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"mailnest-be/internal/storage"
)

// 显式启用的合成数据性能测试，绝不读取生产配置、真实账号或真实邮件。
func TestSearchPerformanceSQLite(t *testing.T) { checkPerformance(t, false) }
func TestSearchPerformanceMySQL(t *testing.T)  { checkPerformance(t, true) }
func checkPerformance(t *testing.T, mysql bool) {
	if os.Getenv("MAILNEST_SEARCH_BENCHMARK") != "1" {
		t.Skip("显式设置 MAILNEST_SEARCH_BENCHMARK=1 才运行性能样本")
	}
	f := newFixture(t, mysql)
	const count = 14350
	var bytes int64
	seed := uint32(73)
	for i := 0; i < count; i++ {
		var b strings.Builder
		for j := 0; j < 450; j++ {
			seed = seed*1664525 + 1013904223
			b.WriteRune(rune(0x4e00 + seed%160))
		}
		b.WriteString(strings.Repeat("邮件检索性能样本 project meeting archive update schedule. ", 75))
		if i%25 == 0 {
			b.WriteString(" 资产编码601 ")
		}
		body := b.String()
		bytes += int64(len(body))
		f.add(t, fmt.Sprintf("合成测试邮件 %06d", i), body, false)
	}
	// 模拟升级前的存量，避免把任务积压算作新引擎查询成本。
	for {
		tasks, err := f.store.ListMailSearchTasks(context.Background(), 200)
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) == 0 {
			break
		}
		if err = f.store.CompleteMailSearchTasks(context.Background(), tasks); err != nil {
			t.Fatal(err)
		}
	}
	s := openService(t, f, t.TempDir(), "active")
	s.cfg.BatchSize = 25
	start := time.Now()
	pump(t, s)
	build := time.Since(start)
	q := storage.ListMailMessagesQuery{UserID: f.user, Keyword: "601", Limit: 50, SummaryOnly: true, SystemFolder: "inbox"}
	assertMatches(t, f, s, q)
	c, err := s.Candidates(context.Background(), storage.MailSearchTextQuery{UserID: f.user, Keyword: q.Keyword})
	if err != nil || !c.Available {
		t.Fatalf("benchmark must exercise index: %v %+v", err, c)
	}
	samples := func(indexed bool) []time.Duration {
		if indexed {
			f.store.SetMailSearchProvider(s)
		} else {
			f.store.SetMailSearchProvider(nil)
		}
		result := make([]time.Duration, 9)
		for i := range result {
			start := time.Now()
			_, _, err := f.store.ListMailMessagesByQuery(q)
			if err != nil {
				t.Fatal(err)
			}
			result[i] = time.Since(start)
		}
		sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
		return result
	}
	baseline, indexed := samples(false), samples(true)
	var disk int64
	if err := filepath.WalkDir(s.path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			disk += info.Size()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	t.Logf("合成样本 mysql=%v messages=%d text_bytes=%d candidates=%d build=%s index_bytes=%d LIKE_p50=%s LIKE_p95=%s Bleve_p50=%s Bleve_p95=%s", mysql, count, bytes, len(c.IDs), build, disk, baseline[4], baseline[8], indexed[4], indexed[8])
}
