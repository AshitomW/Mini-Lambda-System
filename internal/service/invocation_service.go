package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"AshitomW/mini-lambda/internal/config"
	"AshitomW/mini-lambda/internal/domain"
	"AshitomW/mini-lambda/internal/metrics"
	"AshitomW/mini-lambda/internal/repository"
	"AshitomW/mini-lambda/internal/runner"
	"github.com/google/uuid"
)

// InvocationService orchestrates synchronous and asynchronous function executions with bounded concurrency.
type InvocationService struct {
	funcRepo repository.FunctionRepository
	invRepo  repository.InvocationRepository
	runner   runner.ContainerRunner
	metrics  metrics.MetricsRecorder
	cfg      config.Config
	sem      chan struct{}
	wg       sync.WaitGroup
	closedMu sync.RWMutex
	isClosed bool
}

// NewInvocationService initializes InvocationService with the provided repositories, runner, and metrics.
func NewInvocationService(
	funcRepo repository.FunctionRepository,
	invRepo repository.InvocationRepository,
	runner runner.ContainerRunner,
	metrics metrics.MetricsRecorder,
	cfg config.Config,
) *InvocationService {
	return &InvocationService{
		funcRepo: funcRepo,
		invRepo:  invRepo,
		runner:   runner,
		metrics:  metrics,
		cfg:      cfg,
		sem:      make(chan struct{}, cfg.MaxConcurrentInvocations),
	}
}

// InvokeSync executes a function synchronously within the configured execution timeout boundaries.
func (s *InvocationService) InvokeSync(ctx context.Context, identifier string, payload []byte, invCtx domain.InvocationContext, timeout time.Duration) (*domain.InvocationResult, error) {
	s.closedMu.RLock()
	if s.isClosed {
		s.closedMu.RUnlock()
		return nil, domain.ErrRunnerUnavailable
	}
	s.closedMu.RUnlock()

	fn, err := s.funcRepo.GetByNameOrID(ctx, identifier)
	if err != nil {
		return nil, err
	}

	execTimeout := s.resolveTimeout(timeout)
	if fn.TimeoutSec > 0 && timeout <= 0 {
		execTimeout = s.resolveTimeout(time.Duration(fn.TimeoutSec) * time.Second)
	}

	if invCtx.InvocationID == "" {
		invCtx.InvocationID = uuid.NewString()
	}
	invCtx.FunctionName = fn.Name
	invCtx.Deadline = time.Now().Add(execTimeout)
	if invCtx.ContentType == "" {
		invCtx.ContentType = "application/json"
	}

	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return nil, domain.ErrCapacityExceeded
	}

	res, err := s.runner.Invoke(ctx, fn, payload, invCtx, execTimeout)
	if err != nil {
		s.metrics.RecordInvocation(fn.Name, string(domain.StatusFailed), 0)
		return nil, err
	}

	s.metrics.RecordInvocation(fn.Name, string(domain.StatusCompleted), res.DurationMs)
	return res, nil
}

// InvokeAsync registers an invocation record and dispatches execution to a background worker.
func (s *InvocationService) InvokeAsync(ctx context.Context, identifier string, payload []byte, invCtx domain.InvocationContext, timeout time.Duration) (domain.AsyncInvocation, error) {
	s.closedMu.RLock()
	if s.isClosed {
		s.closedMu.RUnlock()
		return domain.AsyncInvocation{}, domain.ErrRunnerUnavailable
	}
	s.closedMu.RUnlock()

	fn, err := s.funcRepo.GetByNameOrID(ctx, identifier)
	if err != nil {
		return domain.AsyncInvocation{}, err
	}

	execTimeout := s.resolveTimeout(timeout)
	if fn.TimeoutSec > 0 && timeout <= 0 {
		execTimeout = s.resolveTimeout(time.Duration(fn.TimeoutSec) * time.Second)
	}

	if invCtx.InvocationID == "" {
		invCtx.InvocationID = uuid.NewString()
	}
	invCtx.FunctionName = fn.Name
	if invCtx.ContentType == "" {
		invCtx.ContentType = "application/json"
	}

	inv := domain.AsyncInvocation{
		ID:             invCtx.InvocationID,
		FunctionID:     fn.ID,
		Status:         domain.StatusPending,
		TraceID:        invCtx.TraceID,
		CallerIdentity: invCtx.CallerIdentity,
		Payload:        string(payload),
		MaxRetries:     s.cfg.MaxRetries,
		CreatedAt:      time.Now().UTC(),
	}

	if err := s.invRepo.Create(ctx, inv); err != nil {
		return domain.AsyncInvocation{}, err
	}

	s.wg.Add(1)
	go s.runAsyncWorker(fn, invCtx, payload, execTimeout)

	return inv, nil
}

func (s *InvocationService) runAsyncWorker(fn domain.Function, invCtx domain.InvocationContext, payload []byte, timeout time.Duration) {
	defer s.wg.Done()

	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	bgCtx := context.Background()

	curr, err := s.invRepo.GetByID(bgCtx, invCtx.InvocationID)
	if err == nil {
		curr.Status = domain.StatusRunning
		_ = s.invRepo.Update(bgCtx, curr)
	}

	maxRetries := s.cfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}

	var res *domain.InvocationResult
	var invokeErr error
	var attempt int

	for attempt = 0; attempt <= maxRetries; attempt++ {
		invCtx.Deadline = time.Now().Add(timeout)
		res, invokeErr = s.runner.Invoke(bgCtx, fn, payload, invCtx, timeout)
		if invokeErr == nil {
			break
		}

		if attempt < maxRetries {
			if curr, err := s.invRepo.GetByID(bgCtx, invCtx.InvocationID); err == nil {
				curr.RetryCount = attempt + 1
				curr.Error = invokeErr.Error()
				_ = s.invRepo.Update(bgCtx, curr)
			}
			backoff := time.Duration(1<<attempt) * 50 * time.Millisecond
			if backoff > 2*time.Second {
				backoff = 2 * time.Second
			}
			time.Sleep(backoff)
		}
	}

	now := time.Now().UTC()
	updated, getErr := s.invRepo.GetByID(bgCtx, invCtx.InvocationID)
	if getErr != nil {
		return
	}

	updated.CompletedAt = &now
	if invokeErr != nil {
		updated.Error = invokeErr.Error()
		updated.RetryCount = attempt
		updated.Status = domain.StatusDeadLetter
		s.metrics.RecordInvocation(fn.Name, string(domain.StatusDeadLetter), 0)
	} else {
		updated.Status = domain.StatusCompleted
		updated.Result = res
		s.metrics.RecordInvocation(fn.Name, string(domain.StatusCompleted), res.DurationMs)
	}

	_ = s.invRepo.Update(bgCtx, updated)
}

// GetAsyncInvocation returns the details of an existing async invocation.
func (s *InvocationService) GetAsyncInvocation(ctx context.Context, id string) (domain.AsyncInvocation, error) {
	if strings.TrimSpace(id) == "" {
		return domain.AsyncInvocation{}, domain.ErrInvalidInput
	}
	return s.invRepo.GetByID(ctx, id)
}

// ListAsyncInvocations returns all async invocations.
func (s *InvocationService) ListAsyncInvocations(ctx context.Context) ([]domain.AsyncInvocation, error) {
	return s.invRepo.List(ctx)
}

// ListDeadLetter returns all async invocations currently in the Dead Letter Queue.
func (s *InvocationService) ListDeadLetter(ctx context.Context) ([]domain.AsyncInvocation, error) {
	return s.invRepo.ListDeadLetter(ctx)
}

// RetryDeadLetter requeues an invocation from the Dead Letter Queue for execution.
func (s *InvocationService) RetryDeadLetter(ctx context.Context, invocationID string) (domain.AsyncInvocation, error) {
	s.closedMu.RLock()
	if s.isClosed {
		s.closedMu.RUnlock()
		return domain.AsyncInvocation{}, domain.ErrRunnerUnavailable
	}
	s.closedMu.RUnlock()

	inv, err := s.invRepo.GetByID(ctx, invocationID)
	if err != nil {
		return domain.AsyncInvocation{}, err
	}

	if inv.Status != domain.StatusDeadLetter && inv.Status != domain.StatusFailed {
		return domain.AsyncInvocation{}, domain.ErrInvalidInput
	}

	fn, err := s.funcRepo.GetByID(ctx, inv.FunctionID)
	if err != nil {
		return domain.AsyncInvocation{}, err
	}

	inv.Status = domain.StatusPending
	inv.RetryCount = 0
	inv.Error = ""
	inv.CompletedAt = nil
	if err := s.invRepo.Update(ctx, inv); err != nil {
		return domain.AsyncInvocation{}, err
	}

	execTimeout := s.resolveTimeout(time.Duration(fn.TimeoutSec) * time.Second)
	invCtx := domain.InvocationContext{
		InvocationID:   inv.ID,
		FunctionName:   fn.Name,
		TraceID:        inv.TraceID,
		CallerIdentity: inv.CallerIdentity,
	}

	s.wg.Add(1)
	go s.runAsyncWorker(fn, invCtx, []byte(inv.Payload), execTimeout)

	return inv, nil
}

// Close marks the service as closed and waits for all active background workers to finish.
func (s *InvocationService) Close() error {
	s.closedMu.Lock()
	s.isClosed = true
	s.closedMu.Unlock()

	s.wg.Wait()
	return nil
}

func (s *InvocationService) resolveTimeout(requested time.Duration) time.Duration {
	if requested <= 0 {
		return s.cfg.DefaultTimeout
	}
	if requested > s.cfg.MaxTimeout {
		return s.cfg.MaxTimeout
	}
	return requested
}
