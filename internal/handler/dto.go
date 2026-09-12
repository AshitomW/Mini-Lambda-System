// Package handler provides HTTP endpoints, DTO models, and routing for Mini-Lambda-System.
package handler

import "time"

// RegisterRequest holds the payload required to register a function.
type RegisterRequest struct {
	Name  string `json:"name" binding:"required"`
	Image string `json:"image" binding:"required"`
}

// InvokeRequest represents the payload passed when invoking a function.
type InvokeRequest struct {
	Event   any `json:"event"`
	Timeout int `json:"timeout"`
}

// SyncInvokeResponse represents the immediate response from a synchronous invocation.
type SyncInvokeResponse struct {
	Result    string    `json:"result"`
	Logs      string    `json:"logs,omitempty"`
	Duration  int64     `json:"duration"`
	Timestamp time.Time `json:"timestamp"`
}

// AsyncInvokeResponse represents the response when an async invocation is accepted.
type AsyncInvokeResponse struct {
	InvocationID string    `json:"invocation_id"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// ImageUploadResponse represents the result of a Docker image upload.
type ImageUploadResponse struct {
	Message  string `json:"message"`
	FileSize int64  `json:"file_size"`
}

// ErrorResponse models standard JSON API errors.
type ErrorResponse struct {
	Error string `json:"error"`
}
