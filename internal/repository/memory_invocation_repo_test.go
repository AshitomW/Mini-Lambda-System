package repository_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"AshitomW/mini-lambda/internal/domain"
	"AshitomW/mini-lambda/internal/repository"
)

func TestMemoryInvocationRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryInvocationRepository(0)

	now := time.Now()
	inv := domain.AsyncInvocation{
		ID:         "inv-1",
		FunctionID: "fn-1",
		Status:     domain.StatusPending,
		CreatedAt:  now,
	}

	if err := repo.Create(ctx, inv); err != nil {
		t.Fatalf("failed to create invocation: %v", err)
	}

	got, err := repo.GetByID(ctx, "inv-1")
	if err != nil {
		t.Fatalf("failed to get invocation: %v", err)
	}
	if got.Status != domain.StatusPending {
		t.Fatalf("expected pending status, got %s", got.Status)
	}

	got.Status = domain.StatusRunning
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("failed to update invocation: %v", err)
	}

	updated, err := repo.GetByID(ctx, "inv-1")
	if err != nil {
		t.Fatalf("failed to get updated invocation: %v", err)
	}
	if updated.Status != domain.StatusRunning {
		t.Fatalf("expected running status, got %s", updated.Status)
	}

	_, err = repo.GetByID(ctx, "non-existent")
	if !errors.Is(err, domain.ErrInvocationNotFound) {
		t.Fatalf("expected ErrInvocationNotFound, got %v", err)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("failed to list invocations: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 invocation, got %d", len(list))
	}
}

func TestMemoryInvocationRepositoryConcurrentSafety(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryInvocationRepository(0)

	const workers = 30
	var wg sync.WaitGroup
	wg.Add(workers * 2)

	for i := 0; i < workers; i++ {
		id := fmt.Sprintf("inv-%d", i)
		inv := domain.AsyncInvocation{
			ID:         id,
			FunctionID: "fn-1",
			Status:     domain.StatusPending,
			CreatedAt:  time.Now(),
		}
		_ = repo.Create(ctx, inv)

		go func(invID string) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				curr, err := repo.GetByID(ctx, invID)
				if err == nil {
					curr.Status = domain.StatusRunning
					_ = repo.Update(ctx, curr)
				}
			}
		}(id)

		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = repo.List(ctx)
			}
		}()
	}

	wg.Wait()
}
