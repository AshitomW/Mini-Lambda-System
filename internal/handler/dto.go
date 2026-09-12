// Package handler provides HTTP endpoints, DTO models, and routing for Mini-Lambda-System.
package handler

import (
	"time"

	"AshitomW/mini-lambda/internal/domain"
)

// RegisterRequest holds the payload required to register a function.
type RegisterRequest struct {
	Name       string            `json:"name" binding:"required"`
	Image      string            `json:"image" binding:"required"`
	Env        map[string]string `json:"env,omitempty"`
	MemoryMB   int64             `json:"memory_mb,omitempty"`
	TimeoutSec int               `json:"timeout_sec,omitempty"`
}

// InvokeRequest represents the payload passed when invoking a function.
type InvokeRequest struct {
	Event      any                `json:"event"`
	CloudEvent *domain.CloudEvent `json:"cloud_event,omitempty"`
	Timeout    int                `json:"timeout"`
}

// SyncInvokeResponse represents the immediate response from a synchronous invocation.
type SyncInvokeResponse struct {
	Result       string    `json:"result"`
	Logs         string    `json:"logs,omitempty"`
	Duration     int64     `json:"duration"`
	InvocationID string    `json:"invocation_id,omitempty"`
	TraceID      string    `json:"trace_id,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// AsyncInvokeResponse represents the response when an async invocation is accepted.
type AsyncInvokeResponse struct {
	InvocationID string    `json:"invocation_id"`
	Status       string    `json:"status"`
	TraceID      string    `json:"trace_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// WebhookInvokeResponse represents the result of an HTTP webhook ingestion.
type WebhookInvokeResponse struct {
	Status       string `json:"status"`
	Function     string `json:"function"`
	InvocationID string `json:"invocation_id,omitempty"`
	Result       string `json:"result,omitempty"`
	DurationMs   int64  `json:"duration_ms,omitempty"`
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
