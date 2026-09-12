package domain_test

import (
	"errors"
	"testing"
	"time"

	"AshitomW/mini-lambda/internal/domain"
)

func TestFunctionValidate(t *testing.T) {
	cases := []struct {
		name    string
		fn      domain.Function
		wantErr bool
	}{
		{
			name: "valid function",
			fn: domain.Function{
				Name:  "my-fn",
				Image: "python:3.9-alpine",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			fn: domain.Function{
				Name:  "   ",
				Image: "python:3.9-alpine",
			},
			wantErr: true,
		},
		{
			name: "empty image",
			fn: domain.Function{
				Name:  "my-fn",
				Image: "",
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tc.wantErr && !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestAsyncInvocationClone(t *testing.T) {
	now := time.Now()
	res := &domain.InvocationResult{
		Output:     "hello",
		Logs:       "logs",
		DurationMs: 120,
	}
	inv := domain.AsyncInvocation{
		ID:          "inv-123",
		FunctionID:  "fn-123",
		Status:      domain.StatusCompleted,
		Result:      res,
		Error:       "",
		CreatedAt:   now,
		CompletedAt: &now,
	}

	cloned := inv.Clone()

	if cloned.ID != inv.ID || cloned.Status != inv.Status {
		t.Fatalf("expected cloned fields to match")
	}

	res.Output = "mutated"
	if cloned.Result.Output == "mutated" {
		t.Fatalf("clone was shallow, expected result to be independent")
	}

	later := now.Add(time.Hour)
	inv.CompletedAt = &later
	if cloned.CompletedAt.Equal(later) {
		t.Fatalf("clone was shallow, expected completed_at to be independent")
	}
}

func TestFunctionSanitize(t *testing.T) {
	fn := domain.Function{
		ID:    "fn-1",
		Name:  "secure-fn",
		Image: "alpine",
		Env: map[string]string{
			"DATABASE_URL": "postgres://localhost",
			"API_KEY":      "secret123",
			"AUTH_TOKEN":   "bearer-xyz",
			"APP_PORT":     "8080",
		},
	}

	sanitized := fn.Sanitize()
	if sanitized.Env["API_KEY"] != "********" {
		t.Errorf("expected API_KEY to be redacted, got %s", sanitized.Env["API_KEY"])
	}
	if sanitized.Env["AUTH_TOKEN"] != "********" {
		t.Errorf("expected AUTH_TOKEN to be redacted, got %s", sanitized.Env["AUTH_TOKEN"])
	}
	if sanitized.Env["APP_PORT"] != "8080" {
		t.Errorf("expected APP_PORT to remain 8080, got %s", sanitized.Env["APP_PORT"])
	}
}

func TestInvocationContextToEnvSlice(t *testing.T) {
	ctx := domain.InvocationContext{
		InvocationID:   "inv-999",
		FunctionName:   "test-fn",
		TraceID:        "trace-abc",
		ContentType:    "application/json",
		CallerIdentity: "admin-client",
		Env: map[string]string{
			"CUSTOM_VAR": "val1",
		},
	}

	envs := ctx.ToEnvSlice()
	foundInvocationID := false
	foundTraceID := false
	foundCustom := false

	for _, e := range envs {
		if e == "LAMBDA_INVOCATION_ID=inv-999" {
			foundInvocationID = true
		}
		if e == "LAMBDA_TRACE_ID=trace-abc" {
			foundTraceID = true
		}
		if e == "CUSTOM_VAR=val1" {
			foundCustom = true
		}
	}

	if !foundInvocationID || !foundTraceID || !foundCustom {
		t.Fatalf("expected required envs in slice, got: %v", envs)
	}
}

func TestCloudEventValidate(t *testing.T) {
	valid := domain.CloudEvent{
		SpecVersion: "1.0",
		ID:          "evt-1",
		Source:      "/orders",
		Type:        "order.created",
		Time:        time.Now().Format(time.RFC3339),
		Data:        map[string]any{"order_id": 123},
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("expected valid event, got %v", err)
	}

	invalid := domain.CloudEvent{
		SpecVersion: "",
	}
	if err := invalid.Validate(); err == nil {
		t.Errorf("expected error for empty specversion, got nil")
	}
}
