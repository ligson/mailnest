package workload

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLimitsCancellationAndIdempotentRelease(t *testing.T) {
	g := New(8)
	first, err := g.Background(context.Background(), "count")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if _, err = g.Background(ctx, "count"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("统计未限流: %v", err)
	}
	second, err := g.Background(context.Background(), "import")
	if err != nil {
		t.Fatal(err)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if _, err = g.Background(ctx2, "index"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	first()
	first()
	second()
	second()
	if g.background != 0 || g.counts != 0 {
		t.Fatal("重复释放破坏计数")
	}
	if New(1).limit != 1 || New(4).limit != 1 || New(8).limit != 2 {
		t.Fatal("资源上限错误")
	}
}

func TestInteractivePriorityAndBackgroundFairness(t *testing.T) {
	g := New(8)
	finish := g.Interactive()
	start := time.Now()
	release, err := g.Background(context.Background(), "persist")
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 180*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("交互优先或公平性异常: %s", elapsed)
	}
	release()
	finish()
	finish()
	if g.foreground != 0 {
		t.Fatal("前台重复释放")
	}
	finish = g.Interactive()
	done := make(chan error, 1)
	go func() {
		release, err := g.Background(context.Background(), "index")
		if err == nil {
			release()
		}
		done <- err
	}()
	finish()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("释放后未唤醒")
	}
}
