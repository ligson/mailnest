package storage

import (
	"context"
	"log"
	"strconv"
	"strings"
)

type MailSearchTextQuery struct {
	UserID                       int64
	Keyword, From, Subject, Body string
}
type MailSearchCandidates struct {
	IDs               []int64
	Available, Shadow bool
}
type MailSearchProvider interface {
	Candidates(context.Context, MailSearchTextQuery) (MailSearchCandidates, error)
	ObserveShadow(userID int64, expected []int64, candidates MailSearchCandidates)
	Mode() string
}

// 可选能力，兼容旧 provider；visit 输入只在回调期间有效，调用方不得全量积累。
type MailSearchPageProvider interface {
	CandidatePages(context.Context, MailSearchTextQuery, func([]int64) error) (bool, error)
}

// 仅在应用装配时调用，运行期间不替换 provider；引擎内部负责代际切换。
func (s *Store) SetMailSearchProvider(provider MailSearchProvider) {
	s.searchProvider = provider
	// 装配/测试切换引擎后，不复用前一种引擎的候选或摘要缓存。
	s.db.cacheVersions.bump(cacheMutation{domain: "mail"})
}
func (s *Store) mailSearchCandidates(ctx context.Context, q MailSearchTextQuery) (MailSearchCandidates, error) {
	if s.searchProvider == nil || strings.TrimSpace(q.Keyword+q.From+q.Subject+q.Body) == "" {
		return MailSearchCandidates{}, nil
	}
	value, err := s.mailReads.get(ctx, s.readKey("candidates", q), func(work context.Context) (any, error) { return s.mailSearchCandidatesUncached(work, q) })
	if err != nil {
		return MailSearchCandidates{}, err
	}
	result := value.(MailSearchCandidates)
	result.IDs = append([]int64{}, result.IDs...)
	return result, nil
}

func (s *Store) mailSearchCandidatesUncached(ctx context.Context, q MailSearchTextQuery) (MailSearchCandidates, error) {
	c, err := s.searchProvider.Candidates(ctx, q)
	if ctx.Err() != nil {
		return c, ctx.Err()
	}
	if err != nil {
		log.Printf("邮件索引查询失败，已回退数据库 user_id=%d error=%v", q.UserID, err)
		return MailSearchCandidates{}, nil
	}
	return c, nil
}
func candidateMailWhere(c MailSearchCandidates) string {
	if !c.Available || c.Shadow {
		return ""
	}
	if len(c.IDs) == 0 {
		return " AND 1=0"
	}
	// ID 来自经过 int64 校验的索引/任务，不含用户字符串，避免 SQLite 参数数量限制。
	var b strings.Builder
	b.WriteString(" AND m.id IN (")
	for i, id := range c.IDs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatInt(id, 10))
	}
	b.WriteByte(')')
	return b.String()
}
