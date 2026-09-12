package domain

import (
	"fmt"
	"strings"
	"time"
)

// Function represents a registered serverless function definition.
type Function struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Env           map[string]string `json:"env,omitempty"`
	AllowNetwork  bool              `json:"allow_network,omitempty"`
	WebhookSecret string            `json:"webhook_secret,omitempty"`
	MemoryMB      int64             `json:"memory_mb,omitempty"`
	TimeoutSec    int               `json:"timeout_sec,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at,omitempty"`
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

// Sanitize returns a copy of Function with sensitive environment variables and secrets redacted.
func (f Function) Sanitize() Function {
	cp := f
	if f.WebhookSecret != "" {
		cp.WebhookSecret = "********"
	}
	if f.Env == nil {
		return cp
	}

	sanitizedEnv := make(map[string]string, len(f.Env))
	for k, v := range f.Env {
		upperKey := strings.ToUpper(k)
		if strings.Contains(upperKey, "KEY") ||
			strings.Contains(upperKey, "SECRET") ||
			strings.Contains(upperKey, "TOKEN") ||
			strings.Contains(upperKey, "PASSWORD") ||
			strings.Contains(upperKey, "AUTH") {
			sanitizedEnv[k] = "********"
		} else {
			sanitizedEnv[k] = v
		}
	}
	cp.Env = sanitizedEnv
	return cp
}
