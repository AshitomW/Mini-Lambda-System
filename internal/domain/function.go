package domain

import (
	"fmt"
	"strings"
	"time"
)

// Function represents a registered serverless function definition.
type Function struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	CreatedAt time.Time `json:"created_at"`
}

// Validate checks whether the function contains valid required fields.
func (f Function) Validate() error {
	if strings.TrimSpace(f.Name) == "" {
		return fmt.Errorf("%w: name cannot be empty", ErrInvalidInput)
	}
	if strings.TrimSpace(f.Image) == "" {
		return fmt.Errorf("%w: image cannot be empty", ErrInvalidInput)
	}
	return nil
}
