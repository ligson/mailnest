package storage

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type readEntry struct {
	value   any
	expires time.Time
}
type readFlight struct {
	done       chan struct{}
	cancel     context.CancelFunc
	refs       int
	value      any
	err        error
	persistent bool
	lastPoll   time.Time
	leaseTimer *time.Timer
}

// readCache 只存当前进程的有界摘要/计数；键包含用户、完整过滤条件和提交版本。
// 每个等待者可独立取消，最后一个离开才取消数据库工作，失败结果永不缓存。
type readCache struct {
	mu      sync.Mutex
	entries map[string]readEntry
	flights map[string]*readFlight
}

func (c *readCache) get(ctx context.Context, key string, load func(context.Context) (any, error)) (any, error) {
	return c.getPolicy(ctx, key, load, false)
}

// getDeferred 最多等待 150ms。统计工作由有界 flight 持有，轮询请求结束不撤销统计。
// 原有工作上限仍为 30 秒；完成后返回精确值，而不是延长浏览器请求超时。
func (c *readCache) getDeferred(ctx context.Context, key string, load func(context.Context) (any, error)) (any, bool, error) {
	wait, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	value, err := c.getPolicy(wait, key, load, true)
	if errors.Is(err, context.DeadlineExceeded) && wait.Err() != nil && ctx.Err() == nil {
		return nil, true, nil
	}
	return value, false, err
}

func (c *readCache) getPolicy(ctx context.Context, key string, load func(context.Context) (any, error), persistent bool) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.entries == nil {
		c.entries = map[string]readEntry{}
		c.flights = map[string]*readFlight{}
	}
	if e, ok := c.entries[key]; ok && time.Now().Before(e.expires) {
		c.mu.Unlock()
		return e.value, nil
	}
	f := c.flights[key]
	if f == nil {
		if len(c.flights) >= 64 {
			c.mu.Unlock()
			return nil, errors.New("并发查询较多，请稍后重试")
		}
		work, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		f = &readFlight{done: make(chan struct{}), cancel: cancel}
		c.flights[key] = f
		go func(f *readFlight) {
			value, err := load(work)
			cacheable := err == nil && work.Err() == nil
			cancel()
			c.mu.Lock()
			f.value, f.err = value, err
			if f.leaseTimer != nil {
				f.leaseTimer.Stop()
			}
			if c.flights[key] == f {
				if f.persistent && err != nil {
					// 失败任务状态短暂保留供下一次轮询读取，不缓存为成功的业务结果。
					time.AfterFunc(3*time.Second, func() {
						c.mu.Lock()
						defer c.mu.Unlock()
						if c.flights[key] == f {
							delete(c.flights, key)
						}
					})
				} else {
					delete(c.flights, key)
				}
				if cacheable {
					now := time.Now()
					for k, e := range c.entries {
						if now.After(e.expires) {
							delete(c.entries, k)
						}
					}
					if len(c.entries) >= 128 {
						for k := range c.entries {
							delete(c.entries, k)
							break
						}
					}
					c.entries[key] = readEntry{value: value, expires: now.Add(3 * time.Second)}
				}
			}
			close(f.done)
			c.mu.Unlock()
		}(f)
	}
	if persistent && !f.persistent {
		f.persistent = true
		f.refs++
	}
	if persistent {
		f.lastPoll = time.Now()
		if f.leaseTimer == nil {
			f.leaseTimer = time.AfterFunc(5*time.Second, func() { c.expireDeferred(key, f) })
		}
	}
	f.refs++
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		f.refs--
		if f.refs == 0 && c.flights[key] == f {
			delete(c.flights, key)
			f.cancel()
		}
		c.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-f.done:
		return f.value, f.err
	}
}

// 轮询租约避免快速换关键词/离开页面后，遗留计数继续占用数据库直到工作总上限。
func (c *readCache) expireDeferred(key string, f *readFlight) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.flights[key] != f {
		return
	}
	select {
	case <-f.done:
		return
	default:
	}
	if f.refs > 1 {
		f.lastPoll = time.Now()
	}
	idle := time.Since(f.lastPoll)
	if idle >= 5*time.Second {
		delete(c.flights, key)
		f.cancel()
		return
	}
	f.leaseTimer.Reset(5*time.Second - idle)
}

func (s *Store) readKey(kind string, query any) string {
	encoded, _ := json.Marshal(query)
	domain := "mail"
	if kind == "contacts.resolve" {
		domain = "contacts"
	}
	return fmt.Sprintf("%s/%s/%s/%x", kind, s.db.cacheVersions.key(cacheQueryUser(query), domain), s.MailSearchMode(), sha256.Sum256(encoded))
}

type messageReadResult struct {
	items []MailMessage
	total int
}
type threadReadResult struct {
	items []MailThreadListItem
	total int
}

func (s *Store) ListMailMessagesByQueryContext(ctx context.Context, q ListMailMessagesQuery) ([]MailMessage, int, error) {
	if q.CountOnly {
		q.Offset = 0
		q.Limit = 0
		q.SkipTotal = false
	}
	if !q.SummaryOnly {
		return s.listMailMessagesUncached(ctx, q)
	}
	value, err := s.mailReads.get(ctx, s.readKey("messages", q), func(work context.Context) (any, error) {
		items, total, err := s.listMailMessagesUncached(work, q)
		return messageReadResult{items, total}, err
	})
	if err != nil {
		return nil, 0, err
	}
	result := value.(messageReadResult)
	return append([]MailMessage{}, result.items...), result.total, nil
}

func (s *Store) ListMailThreadsContext(ctx context.Context, q ListMailThreadsQuery) ([]MailThreadListItem, int, error) {
	if q.CountOnly {
		q.Offset = 0
		q.Limit = 0
		q.SkipTotal = false
	}
	value, err := s.mailReads.get(ctx, s.readKey("threads", q), func(work context.Context) (any, error) {
		items, total, err := s.listMailThreadsUncached(work, q)
		return threadReadResult{items, total}, err
	})
	if err != nil {
		return nil, 0, err
	}
	result := value.(threadReadResult)
	items := append([]MailThreadListItem{}, result.items...)
	for i := range items {
		items[i].Participants = append([]string{}, items[i].Participants...)
	}
	return items, result.total, nil
}
