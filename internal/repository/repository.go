// Package repository defines data storage interfaces and implementations for functions and invocations.
package repository

import (
	"context"

	"AshitomW/mini-lambda/internal/domain"
)

// FunctionRepository provides persistence operations for registered functions.
type FunctionRepository interface {
	Save(ctx context.Context, fn domain.Function) error
	GetByID(ctx context.Context, id string) (domain.Function, error)
	List(ctx context.Context) ([]domain.Function, error)
}

// InvocationRepository provides persistence and retrieval for async invocation records.
type InvocationRepository interface {
	Create(ctx context.Context, inv domain.AsyncInvocation) error
	GetByID(ctx context.Context, id string) (domain.AsyncInvocation, error)
	Update(ctx context.Context, inv domain.AsyncInvocation) error
	List(ctx context.Context) ([]domain.AsyncInvocation, error)
}
