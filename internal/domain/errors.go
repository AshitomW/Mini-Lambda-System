// Package domain defines core domain models, enums, and sentinel errors for the serverless execution engine.
package domain

import "errors"

var (
	// ErrFunctionNotFound indicates that the requested function does not exist.
	ErrFunctionNotFound = errors.New("function not found")

	// ErrInvocationNotFound indicates that the requested invocation does not exist.
	ErrInvocationNotFound = errors.New("invocation not found")

	// ErrInvalidInput indicates that the provided request payload or parameters are invalid.
	ErrInvalidInput = errors.New("invalid input")

	// ErrExecutionTimeout indicates that function execution exceeded the configured timeout.
	ErrExecutionTimeout = errors.New("execution timeout exceeded")

	// ErrCapacityExceeded indicates that the runner reached maximum concurrent invocation capacity.
	ErrCapacityExceeded = errors.New("system concurrency limit reached")

	// ErrRunnerUnavailable indicates that the container runtime is unavailable or failed to initialize.
	ErrRunnerUnavailable = errors.New("container runner is unavailable")
)
