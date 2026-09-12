package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"AshitomW/mini-lambda/internal/domain"
)

// FileFunctionRepository implements FunctionRepository using an in-memory map backed by atomic JSON disk persistence.
type FileFunctionRepository struct {
	mu       sync.RWMutex
	filePath string
	dataDir  string
	items    map[string]domain.Function
}

// NewFileFunctionRepository initializes the repository and loads any existing functions from disk.
func NewFileFunctionRepository(dataDir string) (*FileFunctionRepository, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	repo := &FileFunctionRepository{
		dataDir:  dataDir,
		filePath: filepath.Join(dataDir, "functions.json"),
		items:    make(map[string]domain.Function),
	}

	if err := repo.load(); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *FileFunctionRepository) load() error {
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read functions file: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	if err := json.Unmarshal(data, &r.items); err != nil {
		return fmt.Errorf("failed to parse functions file: %w", err)
	}

	return nil
}

// Save stores the function in memory and commits changes atomically to disk.
func (r *FileFunctionRepository) Save(_ context.Context, fn domain.Function) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.items[fn.ID] = fn

	return r.persist()
}

func (r *FileFunctionRepository) persist() error {
	data, err := json.MarshalIndent(r.items, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal functions: %w", err)
	}

	tmpFile := filepath.Join(r.dataDir, fmt.Sprintf("functions.%d.tmp", os.Getpid()))
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpFile, r.filePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to rename temp file to target: %w", err)
	}

	return nil
}

// GetByID returns the function matching the provided identifier.
func (r *FileFunctionRepository) GetByID(_ context.Context, id string) (domain.Function, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	fn, ok := r.items[id]
	if !ok {
		return domain.Function{}, domain.ErrFunctionNotFound
	}
	return fn, nil
}

// GetByNameOrID returns the function matching either the provided UUID or the function name.
func (r *FileFunctionRepository) GetByNameOrID(_ context.Context, identifier string) (domain.Function, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if fn, ok := r.items[identifier]; ok {
		return fn, nil
	}

	for _, fn := range r.items {
		if fn.Name == identifier {
			return fn, nil
		}
	}

	return domain.Function{}, domain.ErrFunctionNotFound
}

// List returns all registered functions.
func (r *FileFunctionRepository) List(_ context.Context) ([]domain.Function, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]domain.Function, 0, len(r.items))
	for _, fn := range r.items {
		list = append(list, fn)
	}
	return list, nil
}
