package runner_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"AshitomW/mini-lambda/internal/domain"
	"AshitomW/mini-lambda/internal/runner"
)

func TestWarmPoolManager(t *testing.T) {
	var execCount int64
	mock := runner.NewMockRunner()
	mock.InvokeFunc = func(_ context.Context, _ domain.Function, payload []byte, _ domain.InvocationContext, _ time.Duration) (*domain.InvocationResult, error) {
		atomic.AddInt64(&execCount, 1)
		return &domain.InvocationResult{
			Output:     "res:" + string(payload),
			DurationMs: 10,
		}, nil
	}

	idleTimeout := 100 * time.Millisecond
	pool := runner.NewWarmPoolManager(mock, idleTimeout, 2)
	defer pool.Close()

	ctx := context.Background()
	fn := domain.Function{ID: "fn-pool-1", Name: "pool-fn", Image: "alpine"}
	invCtx := domain.InvocationContext{TraceID: "t-1"}

	// 1st invocation: Cold start
	res1, isWarm1, err := pool.Invoke(ctx, fn, []byte("call-1"), invCtx, 2*time.Second)
	if err != nil {
		t.Fatalf("Invoke 1 failed: %v", err)
	}
	if isWarm1 {
		t.Errorf("expected 1st call to be cold start, got isWarm=true")
	}
	if res1.Output != "res:call-1" {
		t.Errorf("unexpected output: %s", res1.Output)
	}
	if pool.ColdStarts != 1 || pool.WarmStarts != 0 {
		t.Errorf("expected 1 cold start and 0 warm starts, got cold=%d warm=%d", pool.ColdStarts, pool.WarmStarts)
	}

	// Active warm worker should now be in standby
	if count := pool.GetActiveWarmCount(fn.ID); count != 1 {
		t.Errorf("expected 1 active warm worker in standby, got %d", count)
	}

	// 2nd invocation: Warm start (worker reused!)
	res2, isWarm2, err := pool.Invoke(ctx, fn, []byte("call-2"), invCtx, 2*time.Second)
	if err != nil {
		t.Fatalf("Invoke 2 failed: %v", err)
	}
	if !isWarm2 {
		t.Errorf("expected 2nd call to be warm start, got isWarm=false")
	}
	if res2.Output != "res:call-2" {
		t.Errorf("unexpected output: %s", res2.Output)
	}
	if pool.ColdStarts != 1 || pool.WarmStarts != 1 {
		t.Errorf("expected 1 cold start and 1 warm start, got cold=%d warm=%d", pool.ColdStarts, pool.WarmStarts)
	}

	// Wait for idle reaper to clean up (scale-to-zero)
	time.Sleep(200 * time.Millisecond)

	if active := pool.GetActiveWarmCount(fn.ID); active != 0 {
		t.Errorf("expected 0 active warm workers after idle timeout, got %d", active)
	}
}
