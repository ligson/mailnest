package searchindex

import (
	"context"
	"fmt"
	"testing"

	"mailnest-be/internal/storage"
)

func TestCandidatePagesBeyond128AndOverflow(t *testing.T) {
	f := newFixture(t, false)
	for i := 0; i < 270; i++ {
		f.add(t, fmt.Sprintf("分页中文 %d", i), "分批候选 601", false)
	}
	s := openService(t, f, t.TempDir(), "active")
	pump(t, s)
	c, err := s.Candidates(context.Background(), storage.MailSearchTextQuery{UserID: f.user, Keyword: "601"})
	if err != nil || !c.Available || len(c.IDs) != 270 {
		t.Fatalf("分批候选漏信: %d %v", len(c.IDs), err)
	}
	seen := map[int64]bool{}
	for _, id := range c.IDs {
		if seen[id] {
			t.Fatal("候选重复", id)
		}
		seen[id] = true
	}
	assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: "601", Limit: 20, Offset: 250})
	assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: "601", Limit: 20, Offset: 250, SkipTotal: true})
	assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: "601", Limit: 20, SkipTotal: true})
	visited, maxBatch := map[int64]bool{}, 0
	available, err := s.CandidatePages(context.Background(), storage.MailSearchTextQuery{UserID: f.user, Keyword: "601"}, func(ids []int64) error {
		maxBatch = max(maxBatch, len(ids))
		for _, id := range ids {
			if visited[id] {
				t.Fatal("流式候选重复", id)
			}
			visited[id] = true
		}
		return nil
	})
	if err != nil || !available || len(visited) != 270 || maxBatch > 128 {
		t.Fatalf("流式遍历不完整: count=%d batch=%d err=%v", len(visited), maxBatch, err)
	}
	s.cfg.MaxCandidates = 200
	c, err = s.Candidates(context.Background(), storage.MailSearchTextQuery{UserID: f.user, Keyword: "601"})
	if err != nil || c.Available || len(c.IDs) != 0 {
		t.Fatalf("宽泛搜索应完整回退而不是截断: %+v %v", c, err)
	}
	assertMatches(t, f, s, storage.ListMailMessagesQuery{Keyword: "601", Limit: 20, SkipTotal: true})
}
