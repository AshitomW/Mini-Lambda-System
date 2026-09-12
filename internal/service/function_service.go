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
	return s.RegisterFunction(ctx, domain.Function{
		Name:  name,
		Image: image,
	})
}

// RegisterFunction validates and stores a function definition with full configuration support.
func (s *FunctionService) RegisterFunction(ctx context.Context, fn domain.Function) (domain.Function, error) {
	if fn.ID == "" {
		fn.ID = uuid.NewString()
	}
	fn.Name = strings.TrimSpace(fn.Name)
	fn.Image = strings.TrimSpace(fn.Image)
	now := time.Now().UTC()
	fn.CreatedAt = now
	fn.UpdatedAt = now

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

// GetByNameOrID looks up a function by either its unique UUID or name.
func (s *FunctionService) GetByNameOrID(ctx context.Context, identifier string) (domain.Function, error) {
	if strings.TrimSpace(identifier) == "" {
		return domain.Function{}, domain.ErrInvalidInput
	}
	return s.repo.GetByNameOrID(ctx, identifier)
}

// List returns all registered functions.
func (s *FunctionService) List(ctx context.Context) ([]domain.Function, error) {
	return s.repo.List(ctx)
}
