// Package workload 协调单进程交互读取与后台小批工作，不持有数据库连接等待排队。
package workload

import (
	"context"
	"sync"
	"time"
)

type Governor struct {
	mu                                    sync.Mutex
	changed                               chan struct{}
	foreground, background, counts, limit int
}

func New(maxConnections int) *Governor {
	limit := 2
	if maxConnections <= 4 {
		limit = 1
	}
	return &Governor{changed: make(chan struct{}), limit: limit}
}

func (g *Governor) signal() { close(g.changed); g.changed = make(chan struct{}) }

func (g *Governor) Interactive() func() {
	if g == nil {
		return func() {}
	}
	g.mu.Lock()
	g.foreground++
	g.mu.Unlock()
	var once sync.Once
	return func() { once.Do(func() { g.mu.Lock(); g.foreground--; g.signal(); g.mu.Unlock() }) }
}

// 新批次优先让交互读取先行；最多让行 200ms 防止持续访问饿死导入和索引。
// 只约束后台并发，不能抢占已开始的 SQL；批次本身仍必须足够小。
func (g *Governor) Background(ctx context.Context, kind string) (func(), error) {
	if g == nil {
		return func() {}, ctx.Err()
	}
	start := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		g.mu.Lock()
		if g.background < g.limit && (kind != "count" || g.counts == 0) && (g.foreground == 0 || time.Since(start) >= 200*time.Millisecond) {
			g.background++
			if kind == "count" {
				g.counts++
			}
			g.mu.Unlock()
			var once sync.Once
			return func() {
				once.Do(func() {
					g.mu.Lock()
					g.background--
					if kind == "count" {
						g.counts--
					}
					g.signal()
					g.mu.Unlock()
				})
			}, nil
		}
		changed := g.changed
		g.mu.Unlock()
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-changed:
			timer.Stop()
		case <-timer.C:
		}
	}
}
