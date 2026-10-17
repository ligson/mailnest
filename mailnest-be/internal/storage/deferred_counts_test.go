package storage

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestDeferredReadSurvivesPollingAndReportsErrors(t *testing.T) {
	var cache readCache
	var calls atomic.Int32
	release := make(chan struct{})
	loader := func(ctx context.Context) (any, error) {
		calls.Add(1)
		select {
		case <-release:
			return 73, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	start := time.Now()
	_, pending, err := cache.getDeferred(context.Background(), "count", loader)
	if err != nil || !pending || time.Since(start) > time.Second {
		t.Fatalf("统计未快速返回等待状态 %v %v", pending, err)
	}
	_, pending, err = cache.getDeferred(context.Background(), "count", loader)
	if err != nil || !pending || calls.Load() != 1 {
		t.Fatal("轮询重复创建任务")
	}
	close(release)
	value, pending, err := cache.getDeferred(context.Background(), "count", loader)
	if err != nil || pending || value != 73 {
		t.Fatalf("统计结果 %v %v %v", value, pending, err)
	}
	var failures int
	fail := func(context.Context) (any, error) { failures++; return nil, errors.New("统计失败") }
	for i := 0; i < 2; i++ {
		_, pending, err = cache.getDeferred(context.Background(), "failure", fail)
		if err == nil || pending {
			t.Fatal("失败状态丢失")
		}
	}
	if failures != 1 {
		t.Fatal("后台失败任务应短暂保留供轮询查看")
	}
}

func TestDeferredReadExpiresWithoutPolling(t *testing.T) {
	var cache readCache
	stopped := make(chan struct{})
	_, pending, err := cache.getDeferred(context.Background(), "orphan", func(ctx context.Context) (any, error) { <-ctx.Done(); close(stopped); return nil, ctx.Err() })
	if err != nil || !pending {
		t.Fatal("没有建立异步工作")
	}
	cache.mu.Lock()
	flight := cache.flights["orphan"]
	flight.lastPoll = time.Now().Add(-6 * time.Second)
	cache.mu.Unlock()
	cache.expireDeferred("orphan", flight)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("停止轮询后未释放数据库工作")
	}
}
