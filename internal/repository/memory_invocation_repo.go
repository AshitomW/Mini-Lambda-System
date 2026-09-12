package repository

import (
	"context"
	"sync"
	"time"

	"AshitomW/mini-lambda/internal/domain"
)

// MemoryInvocationRepository provides an in-memory, thread-safe implementation of InvocationRepository.
type MemoryInvocationRepository struct {
	mu    sync.RWMutex
	items map[string]domain.AsyncInvocation
	ttl   time.Duration
}

// NewMemoryInvocationRepository instantiates a new MemoryInvocationRepository with an optional eviction TTL.
func NewMemoryInvocationRepository(ttl time.Duration) *MemoryInvocationRepository {
	return &MemoryInvocationRepository{
		items: make(map[string]domain.AsyncInvocation),
		ttl:   ttl,
	}
}

// Create stores a new invocation record, saving a deep clone to prevent external data races.
func (r *MemoryInvocationRepository) Create(_ context.Context, inv domain.AsyncInvocation) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.items[inv.ID] = inv.Clone()
	return nil
}

// GetByID retrieves a cloned copy of the invocation record matching the given identifier.
func (r *MemoryInvocationRepository) GetByID(_ context.Context, id string) (domain.AsyncInvocation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	inv, exists := r.items[id]
	if !exists {
		return domain.AsyncInvocation{}, domain.ErrInvocationNotFound
	}

	return inv.Clone(), nil
}

// Update modifies an existing invocation record.
func (r *MemoryInvocationRepository) Update(_ context.Context, inv domain.AsyncInvocation) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[inv.ID]; !exists {
		return domain.ErrInvocationNotFound
	}

	r.items[inv.ID] = inv.Clone()
	return nil
}

// List returns a slice containing cloned copies of all recorded invocations.
func (r *MemoryInvocationRepository) List(_ context.Context) ([]domain.AsyncInvocation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.AsyncInvocation, 0, len(r.items))
	for _, inv := range r.items {
		result = append(result, inv.Clone())
	}

	return result, nil
}

// ListDeadLetter returns all async invocations that have been routed to the Dead Letter Queue.
func (r *MemoryInvocationRepository) ListDeadLetter(_ context.Context) ([]domain.AsyncInvocation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var dlq []domain.AsyncInvocation
	for _, inv := range r.items {
		if inv.Status == domain.StatusDeadLetter {
			dlq = append(dlq, inv.Clone())
		}
	}
	return dlq, nil
}
