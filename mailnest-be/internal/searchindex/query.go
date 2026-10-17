package searchindex

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"mailnest-be/internal/storage"
)

func term(value, field string) query.Query {
	q := bleve.NewTermQuery(value)
	q.SetField(field)
	return q
}
func fieldQuery(field string, grams []string) query.Query {
	required := make([]query.Query, 0, len(grams))
	for _, g := range grams {
		required = append(required, term(g, field))
	}
	return bleve.NewDisjunctionQuery(bleve.NewConjunctionQuery(required...), term(field, "uncertain"))
}
func textQuery(q storage.MailSearchTextQuery) (query.Query, bool) {
	conditions := []query.Query{term(strconv.FormatInt(q.UserID, 10), "user")}
	used := false
	for i, value := range []string{q.Keyword, q.From, q.Subject, q.Body} {
		if strings.Contains(value, `\`) {
			return nil, false
		}
		grams := queryGrams(value)
		if len(grams) == 0 {
			continue
		}
		used = true
		if i == 0 {
			fields := make([]query.Query, 0, len(contentFields))
			for _, field := range contentFields {
				fields = append(fields, fieldQuery(field, grams))
			}
			conditions = append(conditions, bleve.NewDisjunctionQuery(fields...))
		} else {
			conditions = append(conditions, fieldQuery([]string{"", "from", "subject", "body"}[i], grams))
		}
	}
	return bleve.NewConjunctionQuery(conditions...), used
}
func (s *Service) Candidates(ctx context.Context, q storage.MailSearchTextQuery) (storage.MailSearchCandidates, error) {
	result := storage.MailSearchCandidates{Shadow: s.cfg.Mode == "shadow"}
	available, err := s.visitCandidates(ctx, q, func(ids []int64) error { result.IDs = append(result.IDs, ids...); return nil })
	if err != nil || !available {
		result.IDs = nil
		return result, err
	}
	result.Available = true
	return result, nil
}

// CandidatePages 给摘要列表逐批复核；只保留当前命中批次和有界 pending overlay。
// 不能向调用者承诺局部结果：出现溢出/索引不可用时必须丢弃已累积结果并准确回退。
func (s *Service) CandidatePages(ctx context.Context, q storage.MailSearchTextQuery, visit func([]int64) error) (bool, error) {
	if s.cfg.Mode != "active" {
		return false, nil
	}
	return s.visitCandidates(ctx, q, visit)
}

func (s *Service) visitCandidates(ctx context.Context, q storage.MailSearchTextQuery, visit func([]int64) error) (bool, error) {
	compiled, ok := textQuery(q)
	if !ok || q.UserID <= 0 {
		return false, nil
	}
	// 必须先读待处理事件，再读索引；worker 确认事件之前的更新不能遗漏。
	pending, overflow, err := s.store.PendingMailSearchMessageIDs(ctx, q.UserID, s.cfg.MaxCandidates)
	if err != nil || overflow {
		return false, err
	}
	pendingSet := make(map[int64]bool, len(pending))
	for _, id := range pending {
		pendingSet[id] = true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil || s.closed {
		return false, nil
	}
	request := bleve.NewSearchRequestOptions(compiled, 128, 0, false)
	request.Score = "none"
	request.SortBy([]string{"_id"})
	prefix := strconv.FormatInt(q.UserID, 10) + "/"
	visited := len(pendingSet)
	for {
		found, err := s.active.SearchInContext(ctx, request)
		if err != nil {
			return false, err
		}
		if found.Status.Failed > 0 {
			return false, fmt.Errorf("搜索索引分片失败")
		}
		if found.Total > uint64(s.cfg.MaxCandidates) {
			return false, nil
		}
		batch := make([]int64, 0, len(found.Hits))
		for _, hit := range found.Hits {
			if !strings.HasPrefix(hit.ID, prefix) {
				return false, fmt.Errorf("索引用户隔离校验失败")
			}
			id, err := strconv.ParseInt(strings.TrimPrefix(hit.ID, prefix), 10, 64)
			if err != nil || id <= 0 {
				return false, fmt.Errorf("索引邮件 ID 不合法")
			}
			if !pendingSet[id] {
				batch = append(batch, id)
				visited++
			}
		}
		if visited > s.cfg.MaxCandidates {
			return false, nil
		}
		if len(batch) > 0 {
			if err = visit(batch); err != nil {
				return false, err
			}
		}
		if len(found.Hits) < request.Size {
			break
		}
		last := found.Hits[len(found.Hits)-1].Sort
		if len(last) != 1 || (len(request.SearchAfter) > 0 && last[0] <= request.SearchAfter[0]) {
			return false, fmt.Errorf("索引分页位置未推进")
		}
		request.SearchAfter = append([]string{}, last...)
	}
	batch := make([]int64, 0, 128)
	for id := range pendingSet {
		batch = append(batch, id)
		if len(batch) == 128 {
			if err = visit(batch); err != nil {
				return false, err
			}
			batch = make([]int64, 0, 128)
		}
	}
	if len(batch) > 0 {
		if err = visit(batch); err != nil {
			return false, err
		}
	}
	return true, nil
}
func (s *Service) ObserveShadow(userID int64, expected []int64, c storage.MailSearchCandidates) {
	if !c.Shadow || !c.Available {
		return
	}
	seen := map[int64]bool{}
	for _, id := range c.IDs {
		seen[id] = true
	}
	s.shadowChecks.Add(1)
	for _, id := range expected {
		if !seen[id] {
			s.shadowMissing.Add(1)
		}
	}
}
