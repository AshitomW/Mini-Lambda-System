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
	ID             string            `json:"id"`
	FunctionID     string            `json:"function_id"`
	Status         InvocationStatus  `json:"status"`
	Result         *InvocationResult `json:"result,omitempty"`
	Error          string            `json:"error,omitempty"`
	TraceID        string            `json:"trace_id,omitempty"`
	CallerIdentity string            `json:"caller_identity,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	CompletedAt    *time.Time        `json:"completed_at,omitempty"`
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

// InvocationContext carries execution context, tracing, deadlines, and runtime configuration into containers.
type InvocationContext struct {
	InvocationID   string
	FunctionName   string
	Deadline       time.Time
	TraceID        string
	ContentType    string
	CallerIdentity string
	Env            map[string]string
}

// ToEnvSlice converts the invocation context and function variables into Docker/container environment strings.
func (c InvocationContext) ToEnvSlice() []string {
	envs := make([]string, 0, len(c.Env)+8)
	if c.InvocationID != "" {
		envs = append(envs, "MINI_LAMBDA_INVOCATION_ID="+c.InvocationID, "LAMBDA_INVOCATION_ID="+c.InvocationID)
	}
	if c.FunctionName != "" {
		envs = append(envs, "MINI_LAMBDA_FUNCTION_NAME="+c.FunctionName, "LAMBDA_FUNCTION_NAME="+c.FunctionName)
	}
	if !c.Deadline.IsZero() {
		envs = append(envs, "LAMBDA_DEADLINE_MS="+time.Duration(c.Deadline.UnixMilli()).String()) // standard representation
	}
	if c.TraceID != "" {
		envs = append(envs, "MINI_LAMBDA_TRACE_ID="+c.TraceID, "LAMBDA_TRACE_ID="+c.TraceID)
	}
	if c.ContentType != "" {
		envs = append(envs, "MINI_LAMBDA_CONTENT_TYPE="+c.ContentType, "LAMBDA_CONTENT_TYPE="+c.ContentType)
	}
	if c.CallerIdentity != "" {
		envs = append(envs, "MINI_LAMBDA_CALLER_IDENTITY="+c.CallerIdentity, "LAMBDA_CALLER_IDENTITY="+c.CallerIdentity)
	}
	for k, v := range c.Env {
		envs = append(envs, k+"="+v)
	}
	return envs
}

// CloudEvent represents a CNCF CloudEvents 1.0 standard envelope for serverless event delivery.
type CloudEvent struct {
	SpecVersion string `json:"specversion"`
	ID          string `json:"id"`
	Source      string `json:"source"`
	Type        string `json:"type"`
	Time        string `json:"time"`
	ContentType string `json:"datacontenttype,omitempty"`
	Data        any    `json:"data"`
}

// Validate checks that required CloudEvents 1.0 fields are set.
func (ce CloudEvent) Validate() error {
	if ce.SpecVersion == "" {
		return ErrInvalidInput
	}
	if ce.ID == "" || ce.Source == "" || ce.Type == "" {
		return ErrInvalidInput
	}
	return nil
}
