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
func (s *InvocationService) InvokeSync(ctx context.Context, functionID string, payload []byte, timeout time.Duration) (*domain.InvocationResult, error) {
	s.closedMu.RLock()
	if s.isClosed {
		s.closedMu.RUnlock()
		return nil, domain.ErrRunnerUnavailable
	}
	s.closedMu.RUnlock()

	fn, err := s.funcRepo.GetByID(ctx, functionID)
	if err != nil {
		return nil, err
	}

	execTimeout := s.resolveTimeout(timeout)

	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return nil, domain.ErrCapacityExceeded
	}

	res, err := s.runner.Invoke(ctx, fn, payload, execTimeout)
	if err != nil {
		s.metrics.RecordInvocation(fn.Name, string(domain.StatusFailed), 0)
		return nil, err
	}

	s.metrics.RecordInvocation(fn.Name, string(domain.StatusCompleted), res.DurationMs)
	return res, nil
}

// InvokeAsync registers an invocation record and dispatches execution to a background worker.
func (s *InvocationService) InvokeAsync(ctx context.Context, functionID string, payload []byte, timeout time.Duration) (domain.AsyncInvocation, error) {
	s.closedMu.RLock()
	if s.isClosed {
		s.closedMu.RUnlock()
		return domain.AsyncInvocation{}, domain.ErrRunnerUnavailable
	}
	s.closedMu.RUnlock()

	fn, err := s.funcRepo.GetByID(ctx, functionID)
	if err != nil {
		return domain.AsyncInvocation{}, err
	}

	execTimeout := s.resolveTimeout(timeout)

	inv := domain.AsyncInvocation{
		ID:         uuid.NewString(),
		FunctionID: fn.ID,
		Status:     domain.StatusPending,
		CreatedAt:  time.Now().UTC(),
	}

	if err := s.invRepo.Create(ctx, inv); err != nil {
		return domain.AsyncInvocation{}, err
	}

	s.wg.Add(1)
	go s.runAsyncWorker(fn, inv.ID, payload, execTimeout)

	return inv, nil
}

func (s *InvocationService) runAsyncWorker(fn domain.Function, invID string, payload []byte, timeout time.Duration) {
	defer s.wg.Done()

	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	bgCtx := context.Background()

	curr, err := s.invRepo.GetByID(bgCtx, invID)
	if err == nil {
		curr.Status = domain.StatusRunning
		_ = s.invRepo.Update(bgCtx, curr)
	}

	res, err := s.runner.Invoke(bgCtx, fn, payload, timeout)

	now := time.Now().UTC()
	updated, getErr := s.invRepo.GetByID(bgCtx, invID)
	if getErr != nil {
		return
	}

	updated.CompletedAt = &now
	if err != nil {
		updated.Status = domain.StatusFailed
		updated.Error = err.Error()
		s.metrics.RecordInvocation(fn.Name, string(domain.StatusFailed), 0)
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
