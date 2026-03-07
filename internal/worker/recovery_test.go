package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunWithRecovery_PanicRestart(t *testing.T) {
	var callCount atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	reg := NewRegistry()

	done := make(chan struct{})
	go func() {
		RunWithRecovery(ctx, "test-worker", func(ctx context.Context) {
			n := callCount.Add(1)
			if n <= 2 {
				panic("test panic")
			}
			// On 3rd call, just wait for ctx cancellation
			<-ctx.Done()
		}, 50*time.Millisecond, reg)
		close(done)
	}()

	<-done
	count := callCount.Load()
	if count < 3 {
		t.Errorf("expected at least 3 calls (2 panics + 1 normal), got %d", count)
	}
}

func TestRunWithRecovery_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		RunWithRecovery(ctx, "cancel-test", func(ctx context.Context) {
			<-ctx.Done()
		}, time.Second, nil)
		close(done)
	}()

	cancel()
	select {
	case <-done:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("RunWithRecovery did not exit after context cancel")
	}
}

func TestRegistry_Heartbeat(t *testing.T) {
	reg := NewRegistry()
	reg.Heartbeat("worker-a")
	reg.Heartbeat("worker-b")

	status := reg.Status()
	if len(status) != 2 {
		t.Fatalf("expected 2 workers, got %d", len(status))
	}
	if !reg.AllHealthy(time.Minute) {
		t.Error("expected all healthy")
	}
}
