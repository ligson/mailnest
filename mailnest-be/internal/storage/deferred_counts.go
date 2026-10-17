package storage

import "context"

func (s *Store) DeferredMessageCount(ctx context.Context, q ListMailMessagesQuery) (int, bool, error) {
	q.CountOnly = true
	q.SkipTotal = false
	q.SummaryOnly = true
	q.Limit = 0
	q.Offset = 0
	value, pending, err := s.mailReads.getDeferred(ctx, s.readKey("messages", q), func(work context.Context) (any, error) {
		release, err := s.workload.Background(work, "count")
		if err != nil {
			return nil, err
		}
		defer release()
		items, total, err := s.listMailMessagesUncached(work, q)
		return messageReadResult{items, total}, err
	})
	if err != nil || pending {
		return 0, pending, err
	}
	return value.(messageReadResult).total, false, nil
}

func (s *Store) DeferredThreadCount(ctx context.Context, q ListMailThreadsQuery) (int, bool, error) {
	q.CountOnly = true
	q.SkipTotal = false
	q.Limit = 0
	q.Offset = 0
	value, pending, err := s.mailReads.getDeferred(ctx, s.readKey("threads", q), func(work context.Context) (any, error) {
		release, err := s.workload.Background(work, "count")
		if err != nil {
			return nil, err
		}
		defer release()
		items, total, err := s.listMailThreadsUncached(work, q)
		return threadReadResult{items, total}, err
	})
	if err != nil || pending {
		return 0, pending, err
	}
	return value.(threadReadResult).total, false, nil
}
