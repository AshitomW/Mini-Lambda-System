// Package runner provides container execution interfaces and implementations.
package runner

import (
	"context"
	"io"
	"time"

	"AshitomW/mini-lambda/internal/domain"
)

// ContainerRunner specifies the contract for running functions and managing images in containers.
type ContainerRunner interface {
	Invoke(ctx context.Context, fn domain.Function, payload []byte, invCtx domain.InvocationContext, timeout time.Duration) (*domain.InvocationResult, error)
	LoadImage(ctx context.Context, reader io.Reader) error
	ListImages(ctx context.Context) ([]string, error)
	Close() error
}
