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
