package runner

import (
	"context"
	"io"
	"sync"
	"time"

	"AshitomW/mini-lambda/internal/domain"
)

// MockRunner provides an in-memory implementation of ContainerRunner for unit testing.
type MockRunner struct {
	mu             sync.Mutex
	InvokeFunc     func(ctx context.Context, fn domain.Function, payload []byte, invCtx domain.InvocationContext, timeout time.Duration) (*domain.InvocationResult, error)
	LoadImageFunc  func(ctx context.Context, reader io.Reader) error
	ListImagesFunc func(ctx context.Context) ([]string, error)
	Invocations    []domain.Function
	LastContext    domain.InvocationContext
}

// NewMockRunner returns an initialized MockRunner instance with standard default behaviors.
func NewMockRunner() *MockRunner {
	return &MockRunner{
		InvokeFunc: func(_ context.Context, _ domain.Function, payload []byte, _ domain.InvocationContext, _ time.Duration) (*domain.InvocationResult, error) {
			return &domain.InvocationResult{
				Output:     string(payload),
				Logs:       "execution logs",
				DurationMs: 15,
			}, nil
		},
		LoadImageFunc: func(_ context.Context, _ io.Reader) error {
			return nil
		},
		ListImagesFunc: func(_ context.Context) ([]string, error) {
			return []string{"alpine:latest", "python:3.9-alpine"}, nil
		},
	}
}

// Invoke records and executes the mock function invocation.
func (m *MockRunner) Invoke(ctx context.Context, fn domain.Function, payload []byte, invCtx domain.InvocationContext, timeout time.Duration) (*domain.InvocationResult, error) {
	m.mu.Lock()
	m.Invocations = append(m.Invocations, fn)
	m.LastContext = invCtx
	m.mu.Unlock()

	if m.InvokeFunc != nil {
		return m.InvokeFunc(ctx, fn, payload, invCtx, timeout)
	}
	return &domain.InvocationResult{Output: "ok", Logs: "logs", DurationMs: 10}, nil
}

// LoadImage simulates loading an image.
func (m *MockRunner) LoadImage(ctx context.Context, reader io.Reader) error {
	if m.LoadImageFunc != nil {
		return m.LoadImageFunc(ctx, reader)
	}
	return nil
}

// ListImages returns mock image tags.
func (m *MockRunner) ListImages(ctx context.Context) ([]string, error) {
	if m.ListImagesFunc != nil {
		return m.ListImagesFunc(ctx)
	}
	return []string{"mock-image:latest"}, nil
}

// Close simulates closing runner resources.
func (m *MockRunner) Close() error {
	return nil
}
