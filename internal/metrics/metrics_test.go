package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"AshitomW/mini-lambda/internal/metrics"
)

func TestPrometheusMetricsRecording(t *testing.T) {
	prom := metrics.NewPrometheusMetrics()

	prom.RecordInvocation("my-func", "COMPLETED", 150)
	prom.RecordInvocation("my-func", "FAILED", 500)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	prom.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from metrics handler, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `mini_lambda_invocations_total{function="my-func",status="COMPLETED"} 1`) {
		t.Errorf("missing completed invocation metric in output: %s", body)
	}
	if !strings.Contains(body, `mini_lambda_invocations_total{function="my-func",status="FAILED"} 1`) {
		t.Errorf("missing failed invocation metric in output: %s", body)
	}
	if !strings.Contains(body, `mini_lambda_invocation_duration_seconds_count{function="my-func"} 2`) {
		t.Errorf("missing duration metric count in output: %s", body)
	}
}

func TestNoopMetrics(t *testing.T) {
	noop := metrics.NewNoopMetrics()
	noop.RecordInvocation("test", "COMPLETED", 100)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	noop.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from noop handler, got %d", rec.Code)
	}
}
