// Package service coordinates business domain workflows, invocation scheduling, and image management.
package service

import (
	"context"
	"strings"
	"time"

	"AshitomW/mini-lambda/internal/domain"
	"AshitomW/mini-lambda/internal/repository"
	"github.com/google/uuid"
)

// FunctionService handles registration, retrieval, and listing of serverless functions.
type FunctionService struct {
	repo repository.FunctionRepository
}

// NewFunctionService instantiates a new FunctionService backed by the given repository.
func NewFunctionService(repo repository.FunctionRepository) *FunctionService {
	return &FunctionService{repo: repo}
}

// Register validates and registers a new function.
func (s *FunctionService) Register(ctx context.Context, name, image string) (domain.Function, error) {
	fn := domain.Function{
		ID:        uuid.NewString(),
		Name:      strings.TrimSpace(name),
		Image:     strings.TrimSpace(image),
		CreatedAt: time.Now().UTC(),
	}

	if err := fn.Validate(); err != nil {
		return domain.Function{}, err
	}

	if err := s.repo.Save(ctx, fn); err != nil {
		return domain.Function{}, err
	}

	return fn, nil
}

// GetByID looks up a function by its identifier.
func (s *FunctionService) GetByID(ctx context.Context, id string) (domain.Function, error) {
	if strings.TrimSpace(id) == "" {
		return domain.Function{}, domain.ErrInvalidInput
	}
	return s.repo.GetByID(ctx, id)
}

// List returns all registered functions.
func (s *FunctionService) List(ctx context.Context) ([]domain.Function, error) {
	return s.repo.List(ctx)
}
