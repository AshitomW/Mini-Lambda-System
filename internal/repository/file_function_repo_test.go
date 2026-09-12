package repository_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"AshitomW/mini-lambda/internal/domain"
	"AshitomW/mini-lambda/internal/repository"
)

func TestFileFunctionRepositoryCRUD(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	repo, err := repository.NewFileFunctionRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	fn := domain.Function{
		ID:        "fn-1",
		Name:      "hello",
		Image:     "python:alpine",
		CreatedAt: time.Now().UTC(),
	}

	if err := repo.Save(ctx, fn); err != nil {
		t.Fatalf("failed to save function: %v", err)
	}

	got, err := repo.GetByID(ctx, "fn-1")
	if err != nil {
		t.Fatalf("failed to get function: %v", err)
	}
	if got.Name != fn.Name || got.Image != fn.Image {
		t.Fatalf("expected function %+v, got %+v", fn, got)
	}

	_, err = repo.GetByID(ctx, "non-existent")
	if !errors.Is(err, domain.ErrFunctionNotFound) {
		t.Fatalf("expected ErrFunctionNotFound, got %v", err)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("failed to list functions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 function, got %d", len(list))
	}

	reloadedRepo, err := repository.NewFileFunctionRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to reload repo: %v", err)
	}
	reloadedFn, err := reloadedRepo.GetByID(ctx, "fn-1")
	if err != nil {
		t.Fatalf("failed to find function in reloaded repo: %v", err)
	}
	if reloadedFn.ID != fn.ID {
		t.Fatalf("expected reloaded function ID %s, got %s", fn.ID, reloadedFn.ID)
	}
}

func TestFileFunctionRepositoryConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	repo, err := repository.NewFileFunctionRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		id := fmt.Sprintf("fn-%d", i)
		go func(fnID string) {
			defer wg.Done()
			fn := domain.Function{
				ID:        fnID,
				Name:      "test-name",
				Image:     "test-image",
				CreatedAt: time.Now(),
			}
			_ = repo.Save(ctx, fn)
			_, _ = repo.GetByID(ctx, fnID)
			_, _ = repo.List(ctx)
		}(id)
	}

	wg.Wait()

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("failed to list functions: %v", err)
	}
	if len(list) != workers {
		t.Fatalf("expected %d functions, got %d", workers, len(list))
	}
}

func TestFileFunctionRepositoryCorruptedFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "functions.json")
	if err := os.WriteFile(filePath, []byte("invalid-json"), 0644); err != nil {
		t.Fatalf("failed to write corrupted file: %v", err)
	}

	_, err := repository.NewFileFunctionRepository(tempDir)
	if err == nil {
		t.Fatalf("expected error loading corrupted file, got nil")
	}
}

func TestFileFunctionRepositoryGetByNameOrID(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	repo, err := repository.NewFileFunctionRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	fn := domain.Function{
		ID:        "550e8400-e29b-41d4-a716-446655440000",
		Name:      "user-service",
		Image:     "python:alpine",
		CreatedAt: time.Now().UTC(),
	}

	if err := repo.Save(ctx, fn); err != nil {
		t.Fatalf("failed to save function: %v", err)
	}

	// Lookup by ID
	byID, err := repo.GetByNameOrID(ctx, "550e8400-e29b-41d4-a716-446655440000")
	if err != nil {
		t.Fatalf("failed to get by ID: %v", err)
	}
	if byID.Name != "user-service" {
		t.Errorf("expected name user-service, got %s", byID.Name)
	}

	// Lookup by Name
	byName, err := repo.GetByNameOrID(ctx, "user-service")
	if err != nil {
		t.Fatalf("failed to get by Name: %v", err)
	}
	if byName.ID != fn.ID {
		t.Errorf("expected ID %s, got %s", fn.ID, byName.ID)
	}

	// Lookup non-existent
	_, err = repo.GetByNameOrID(ctx, "non-existent")
	if !errors.Is(err, domain.ErrFunctionNotFound) {
		t.Errorf("expected ErrFunctionNotFound, got %v", err)
	}
}
