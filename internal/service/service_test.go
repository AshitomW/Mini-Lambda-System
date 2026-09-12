package service_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"AshitomW/mini-lambda/internal/config"
	"AshitomW/mini-lambda/internal/domain"
	"AshitomW/mini-lambda/internal/metrics"
	"AshitomW/mini-lambda/internal/repository"
	"AshitomW/mini-lambda/internal/runner"
	"AshitomW/mini-lambda/internal/service"
)

func setupTestServices(t *testing.T, maxConcurrency int) (*service.FunctionService, *service.InvocationService, *service.ImageService, *runner.MockRunner) {
	t.Helper()

	tempDir := t.TempDir()
	funcRepo, err := repository.NewFileFunctionRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create func repo: %v", err)
	}

	invRepo := repository.NewMemoryInvocationRepository(0)
	mockRunner := runner.NewMockRunner()
	noopMetrics := metrics.NewNoopMetrics()

	cfg := config.Config{
		MaxConcurrentInvocations: maxConcurrency,
		DefaultTimeout:           5 * time.Second,
		MaxTimeout:               10 * time.Second,
	}

	funcService := service.NewFunctionService(funcRepo)
	invService := service.NewInvocationService(funcRepo, invRepo, mockRunner, noopMetrics, cfg)
	imgService := service.NewImageService(mockRunner)

	return funcService, invService, imgService, mockRunner
}

func TestFunctionService(t *testing.T) {
	ctx := context.Background()
	funcService, _, _, _ := setupTestServices(t, 5)

	fn, err := funcService.Register(ctx, "hello-fn", "python:3.9-alpine")
	if err != nil {
		t.Fatalf("failed to register function: %v", err)
	}
	if fn.ID == "" || fn.Name != "hello-fn" {
		t.Fatalf("unexpected function returned: %+v", fn)
	}

	_, err = funcService.Register(ctx, "", "python:3.9-alpine")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	found, err := funcService.GetByID(ctx, fn.ID)
	if err != nil {
		t.Fatalf("failed to find function: %v", err)
	}
	if found.ID != fn.ID {
		t.Fatalf("expected ID %s, got %s", fn.ID, found.ID)
	}

	list, err := funcService.List(ctx)
	if err != nil {
		t.Fatalf("failed to list functions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 function, got %d", len(list))
	}
}

func TestInvocationServiceSync(t *testing.T) {
	ctx := context.Background()
	funcService, invService, _, mockRunner := setupTestServices(t, 5)

	fn, err := funcService.Register(ctx, "test-func", "alpine")
	if err != nil {
		t.Fatalf("failed to register function: %v", err)
	}

	mockRunner.InvokeFunc = func(_ context.Context, _ domain.Function, payload []byte, invCtx domain.InvocationContext, _ time.Duration) (*domain.InvocationResult, error) {
		return &domain.InvocationResult{
			Output:     "processed: " + string(payload) + " trace:" + invCtx.TraceID,
			Logs:       "some logs",
			DurationMs: 42,
		}, nil
	}

	invCtx := domain.InvocationContext{TraceID: "trace-123"}
	res, err := invService.InvokeSync(ctx, fn.ID, []byte(`{"message":"ping"}`), invCtx, 2*time.Second)
	if err != nil {
		t.Fatalf("InvokeSync by ID failed: %v", err)
	}
	if res.Output != `processed: {"message":"ping"} trace:trace-123` {
		t.Fatalf("unexpected output: %s", res.Output)
	}

	// Test invocation by function Name
	resByName, err := invService.InvokeSync(ctx, "test-func", []byte(`{"message":"by-name"}`), domain.InvocationContext{}, 2*time.Second)
	if err != nil {
		t.Fatalf("InvokeSync by name failed: %v", err)
	}
	if resByName.Output != `processed: {"message":"by-name"} trace:` {
		t.Fatalf("unexpected output: %s", resByName.Output)
	}

	_, err = invService.InvokeSync(ctx, "non-existent", []byte(`{}`), domain.InvocationContext{}, 0)
	if !errors.Is(err, domain.ErrFunctionNotFound) {
		t.Fatalf("expected ErrFunctionNotFound, got %v", err)
	}
}

func TestInvocationServiceCapacityLimit(t *testing.T) {
	ctx := context.Background()
	funcService, invService, _, mockRunner := setupTestServices(t, 1)

	fn, err := funcService.Register(ctx, "slow-func", "alpine")
	if err != nil {
		t.Fatalf("failed to register function: %v", err)
	}

	started := make(chan struct{})
	block := make(chan struct{})

	mockRunner.InvokeFunc = func(_ context.Context, _ domain.Function, _ []byte, _ domain.InvocationContext, _ time.Duration) (*domain.InvocationResult, error) {
		close(started)
		<-block
		return &domain.InvocationResult{Output: "done"}, nil
	}

	go func() {
		_, _ = invService.InvokeSync(ctx, fn.ID, []byte(`{}`), domain.InvocationContext{}, 5*time.Second)
	}()

	<-started

	_, err = invService.InvokeSync(ctx, fn.ID, []byte(`{}`), domain.InvocationContext{}, 5*time.Second)
	if !errors.Is(err, domain.ErrCapacityExceeded) {
		t.Fatalf("expected ErrCapacityExceeded when pool saturated, got %v", err)
	}

	close(block)
}

func TestInvocationServiceAsync(t *testing.T) {
	ctx := context.Background()
	funcService, invService, _, mockRunner := setupTestServices(t, 5)

	_, err := funcService.Register(ctx, "async-func", "alpine")
	if err != nil {
		t.Fatalf("failed to register function: %v", err)
	}

	mockRunner.InvokeFunc = func(_ context.Context, _ domain.Function, _ []byte, invCtx domain.InvocationContext, _ time.Duration) (*domain.InvocationResult, error) {
		time.Sleep(20 * time.Millisecond)
		return &domain.InvocationResult{
			Output:     `{"status":"ok","caller":"` + invCtx.CallerIdentity + `"}`,
			Logs:       "execution log",
			DurationMs: 25,
		}, nil
	}

	invCtx := domain.InvocationContext{CallerIdentity: "tester-app"}
	inv, err := invService.InvokeAsync(ctx, "async-func", []byte(`{}`), invCtx, 2*time.Second)
	if err != nil {
		t.Fatalf("InvokeAsync failed: %v", err)
	}
	if inv.Status != domain.StatusPending {
		t.Fatalf("expected initial status PENDING, got %s", inv.Status)
	}

	time.Sleep(100 * time.Millisecond)

	finalInv, err := invService.GetAsyncInvocation(ctx, inv.ID)
	if err != nil {
		t.Fatalf("failed to get async invocation: %v", err)
	}
	if finalInv.Status != domain.StatusCompleted {
		t.Fatalf("expected COMPLETED status, got %s", finalInv.Status)
	}
	if finalInv.Result == nil || finalInv.Result.Output != `{"status":"ok","caller":"tester-app"}` {
		t.Fatalf("unexpected async result: %+v", finalInv.Result)
	}

	list, err := invService.ListAsyncInvocations(ctx)
	if err != nil {
		t.Fatalf("failed to list async invocations: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 invocation, got %d", len(list))
	}

	if err := invService.Close(); err != nil {
		t.Fatalf("invService.Close failed: %v", err)
	}
}

func TestImageService(t *testing.T) {
	ctx := context.Background()
	_, _, imgService, _ := setupTestServices(t, 2)

	dummyTar := bytes.NewReader([]byte("dummy-tar-content"))
	if err := imgService.UploadImage(ctx, dummyTar); err != nil {
		t.Fatalf("UploadImage failed: %v", err)
	}

	imgs, err := imgService.ListImages(ctx)
	if err != nil {
		t.Fatalf("ListImages failed: %v", err)
	}
	if len(imgs) == 0 {
		t.Fatalf("expected at least 1 image tag")
	}
}

func TestInvocationServiceRetryAndDLQ(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	funcRepo, err := repository.NewFileFunctionRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create func repo: %v", err)
	}

	invRepo := repository.NewMemoryInvocationRepository(0)
	mockRunner := runner.NewMockRunner()
	var invokeCount int
	mockRunner.InvokeFunc = func(_ context.Context, _ domain.Function, payload []byte, invCtx domain.InvocationContext, _ time.Duration) (*domain.InvocationResult, error) {
		invokeCount++
		if invokeCount < 3 {
			return nil, errors.New("simulated runtime crash")
		}
		return &domain.InvocationResult{
			Output:     "succeeded on retry: " + string(payload),
			DurationMs: 15,
		}, nil
	}

	noopMetrics := metrics.NewNoopMetrics()
	cfg := config.Config{
		MaxConcurrentInvocations: 5,
		DefaultTimeout:           5 * time.Second,
		MaxTimeout:               10 * time.Second,
		MaxRetries:               2,
	}

	funcService := service.NewFunctionService(funcRepo)
	invService := service.NewInvocationService(funcRepo, invRepo, mockRunner, noopMetrics, cfg)

	fn, err := funcService.Register(ctx, "retry-fn", "alpine")
	if err != nil {
		t.Fatalf("failed to register fn: %v", err)
	}

	// 1. Invocation that succeeds after retries (attempt 1 fails, attempt 2 fails, attempt 3 succeeds)
	inv, err := invService.InvokeAsync(ctx, fn.ID, []byte(`{"job":"retry-success"}`), domain.InvocationContext{}, 0)
	if err != nil {
		t.Fatalf("failed to invoke async: %v", err)
	}

	// Wait for retries (50ms + 100ms backoff)
	time.Sleep(300 * time.Millisecond)

	finalInv, err := invService.GetAsyncInvocation(ctx, inv.ID)
	if err != nil {
		t.Fatalf("failed to get async invocation: %v", err)
	}
	if finalInv.Status != domain.StatusCompleted {
		t.Fatalf("expected COMPLETED status after retries, got %s, err: %s", finalInv.Status, finalInv.Error)
	}
	if finalInv.RetryCount < 2 {
		t.Fatalf("expected at least 2 retries, got %d", finalInv.RetryCount)
	}

	// 2. Invocation that exhausts all retries and transitions to DEAD_LETTER
	mockRunner.InvokeFunc = func(_ context.Context, _ domain.Function, _ []byte, _ domain.InvocationContext, _ time.Duration) (*domain.InvocationResult, error) {
		return nil, errors.New("permanent failure")
	}

	dlqInv, err := invService.InvokeAsync(ctx, fn.ID, []byte(`{"job":"dlq-payload"}`), domain.InvocationContext{}, 0)
	if err != nil {
		t.Fatalf("failed to invoke async: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	failedInv, err := invService.GetAsyncInvocation(ctx, dlqInv.ID)
	if err != nil {
		t.Fatalf("failed to get async invocation: %v", err)
	}
	if failedInv.Status != domain.StatusDeadLetter {
		t.Fatalf("expected DEAD_LETTER status, got %s", failedInv.Status)
	}

	dlqList, err := invService.ListDeadLetter(ctx)
	if err != nil {
		t.Fatalf("failed to list dlq: %v", err)
	}
	if len(dlqList) != 1 || dlqList[0].ID != dlqInv.ID {
		t.Fatalf("expected 1 DLQ invocation with ID %s, got %+v", dlqInv.ID, dlqList)
	}

	// 3. Retry the dead-lettered message with a repaired runner
	mockRunner.InvokeFunc = func(_ context.Context, _ domain.Function, payload []byte, _ domain.InvocationContext, _ time.Duration) (*domain.InvocationResult, error) {
		return &domain.InvocationResult{
			Output:     "replayed successfully: " + string(payload),
			DurationMs: 10,
		}, nil
	}

	retried, err := invService.RetryDeadLetter(ctx, dlqInv.ID)
	if err != nil {
		t.Fatalf("RetryDeadLetter failed: %v", err)
	}
	if retried.Status != domain.StatusPending {
		t.Fatalf("expected PENDING status on retry, got %s", retried.Status)
	}

	time.Sleep(100 * time.Millisecond)

	replayedInv, err := invService.GetAsyncInvocation(ctx, dlqInv.ID)
	if err != nil {
		t.Fatalf("failed to get replayed invocation: %v", err)
	}
	if replayedInv.Status != domain.StatusCompleted {
		t.Fatalf("expected COMPLETED after DLQ replay, got %s", replayedInv.Status)
	}
	if replayedInv.Result == nil || replayedInv.Result.Output != `replayed successfully: {"job":"dlq-payload"}` {
		t.Fatalf("unexpected replayed output: %+v", replayedInv.Result)
	}
}
