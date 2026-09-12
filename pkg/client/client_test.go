package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"AshitomW/mini-lambda/internal/domain"
	"AshitomW/mini-lambda/pkg/client"
)

func setupMockServer(t *testing.T) (*httptest.Server, *client.Client) {
	t.Helper()

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"UP"}`))
	})

	// Functions endpoints
	mux.HandleFunc("/functions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var fn domain.Function
			_ = json.NewDecoder(r.Body).Decode(&fn)
			fn.ID = "fn-uuid-123"
			fn.CreatedAt = time.Now()
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(fn)
			return
		}
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]domain.Function{
				{ID: "fn-1", Name: "hello-fn", Image: "alpine"},
			})
			return
		}
	})

	mux.HandleFunc("/functions/hello-fn", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(domain.Function{
			ID:    "fn-1",
			Name:  "hello-fn",
			Image: "alpine",
		})
	})

	// Invoke endpoint
	mux.HandleFunc("/invoke/hello-fn", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)

		res := client.SyncInvokeResult{
			Result:       `{"message":"success"}`,
			DurationMs:   15,
			InvocationID: "inv-sync-1",
			TraceID:      r.Header.Get("traceparent"),
			Timestamp:    time.Now(),
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(res)
	})

	// Async invoke endpoint
	mux.HandleFunc("/invoke/hello-fn/async", func(w http.ResponseWriter, r *http.Request) {
		res := client.AsyncInvokeResult{
			InvocationID: "inv-async-999",
			Status:       "PENDING",
			CreatedAt:    time.Now(),
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(res)
	})

	// Invocation polling
	var pollCount int
	mux.HandleFunc("/invocations/inv-async-999", func(w http.ResponseWriter, r *http.Request) {
		pollCount++
		status := domain.StatusRunning
		var result *domain.InvocationResult
		if pollCount >= 2 {
			status = domain.StatusCompleted
			result = &domain.InvocationResult{Output: "async done", DurationMs: 30}
		}
		inv := domain.AsyncInvocation{
			ID:         "inv-async-999",
			FunctionID: "fn-1",
			Status:     status,
			Result:     result,
			CreatedAt:  time.Now(),
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(inv)
	})

	// Webhook endpoint
	mux.HandleFunc("/hooks/webhook-target", func(w http.ResponseWriter, r *http.Request) {
		res := client.WebhookResult{
			Status:     "SUCCESS",
			Function:   "webhook-target",
			Result:     "hook processed",
			DurationMs: 8,
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(res)
	})

	server := httptest.NewServer(mux)
	sdkClient := client.NewClient(server.URL)
	return server, sdkClient
}

func TestSDKHealthAndFunctions(t *testing.T) {
	server, sdk := setupMockServer(t)
	defer server.Close()
	ctx := context.Background()

	ok, err := sdk.HealthCheck(ctx)
	if err != nil || !ok {
		t.Fatalf("HealthCheck failed: %v", err)
	}

	created, err := sdk.RegisterFunction(ctx, domain.Function{
		Name:  "new-fn",
		Image: "python:alpine",
		Env: map[string]string{
			"MODE": "test",
		},
	})
	if err != nil {
		t.Fatalf("RegisterFunction failed: %v", err)
	}
	if created.ID != "fn-uuid-123" {
		t.Fatalf("unexpected ID: %s", created.ID)
	}

	fn, err := sdk.GetFunction(ctx, "hello-fn")
	if err != nil {
		t.Fatalf("GetFunction failed: %v", err)
	}
	if fn.Name != "hello-fn" {
		t.Fatalf("unexpected function name: %s", fn.Name)
	}

	list, err := sdk.ListFunctions(ctx)
	if err != nil {
		t.Fatalf("ListFunctions failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 function in list, got %d", len(list))
	}
}

func TestSDKInvokeSyncAndCloudEvent(t *testing.T) {
	server, sdk := setupMockServer(t)
	defer server.Close()
	ctx := context.Background()

	res, err := sdk.InvokeSync(ctx, "hello-fn", map[string]string{"ping": "pong"}, client.InvokeOptions{
		TraceID: "trace-xyz",
	})
	if err != nil {
		t.Fatalf("InvokeSync failed: %v", err)
	}
	if res.Result != `{"message":"success"}` || res.TraceID != "trace-xyz" {
		t.Fatalf("unexpected result: %+v", res)
	}

	ce := domain.CloudEvent{
		SpecVersion: "1.0",
		ID:          "evt-123",
		Source:      "/sdk/test",
		Type:        "com.sdk.event",
		Time:        time.Now().Format(time.RFC3339),
		Data:        map[string]any{"key": "val"},
	}
	ceRes, err := sdk.InvokeCloudEvent(ctx, "hello-fn", ce)
	if err != nil {
		t.Fatalf("InvokeCloudEvent failed: %v", err)
	}
	if ceRes.Result != `{"message":"success"}` {
		t.Fatalf("unexpected ce result: %+v", ceRes)
	}
}

func TestSDKAsyncAndPolling(t *testing.T) {
	server, sdk := setupMockServer(t)
	defer server.Close()
	ctx := context.Background()

	asyncRes, err := sdk.InvokeAsync(ctx, "hello-fn", map[string]string{"task": "job"})
	if err != nil {
		t.Fatalf("InvokeAsync failed: %v", err)
	}
	if asyncRes.InvocationID != "inv-async-999" {
		t.Fatalf("unexpected async ID: %s", asyncRes.InvocationID)
	}

	finalInv, err := sdk.WaitForAsyncResult(ctx, asyncRes.InvocationID, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForAsyncResult failed: %v", err)
	}
	if finalInv.Status != domain.StatusCompleted {
		t.Fatalf("expected completed status, got %s", finalInv.Status)
	}
	if finalInv.Result.Output != "async done" {
		t.Fatalf("unexpected output: %s", finalInv.Result.Output)
	}
}

func TestSDKSendWebhook(t *testing.T) {
	server, sdk := setupMockServer(t)
	defer server.Close()
	ctx := context.Background()

	whRes, err := sdk.SendWebhook(ctx, "webhook-target", []byte(`{"action":"sync"}`), false, map[string]string{
		"X-Custom-Header": "custom-val",
	})
	if err != nil {
		t.Fatalf("SendWebhook failed: %v", err)
	}
	if whRes.Status != "SUCCESS" || whRes.Result != "hook processed" {
		t.Fatalf("unexpected webhook result: %+v", whRes)
	}
}
