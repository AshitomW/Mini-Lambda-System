package domain

import "time"

// InvocationStatus represents the lifecycle state of an invocation.
type InvocationStatus string

const (
	// StatusPending indicates the invocation is queued for processing.
	StatusPending InvocationStatus = "PENDING"

	// StatusRunning indicates the invocation is currently executing.
	StatusRunning InvocationStatus = "RUNNING"

	// StatusCompleted indicates the invocation finished successfully.
	StatusCompleted InvocationStatus = "COMPLETED"

	// StatusFailed indicates the invocation ended with an error or timeout.
	StatusFailed InvocationStatus = "FAILED"
)

// InvocationResult holds output and telemetry produced by a function run.
type InvocationResult struct {
	Output     string `json:"output"`
	Logs       string `json:"logs"`
	DurationMs int64  `json:"duration_ms"`
}

// AsyncInvocation models an asynchronous invocation record.
type AsyncInvocation struct {
	ID          string            `json:"id"`
	FunctionID  string            `json:"function_id"`
	Status      InvocationStatus  `json:"status"`
	Result      *InvocationResult `json:"result,omitempty"`
	Error       string            `json:"error,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
}

// Clone produces a deep copy of the invocation record to avoid data races.
func (a AsyncInvocation) Clone() AsyncInvocation {
	cp := a
	if a.Result != nil {
		resCp := *a.Result
		cp.Result = &resCp
	}
	if a.CompletedAt != nil {
		tCp := *a.CompletedAt
		cp.CompletedAt = &tCp
	}
	return cp
}
