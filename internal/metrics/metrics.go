// Package metrics provides observability primitives for recording invocation telemetry and Prometheus exposition.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsRecorder defines an interface for tracking invocation operations.
type MetricsRecorder interface {
	RecordInvocation(functionName string, status string, durationMs int64)
	Handler() http.Handler
}

// PrometheusMetrics implements MetricsRecorder using the Prometheus client.
type PrometheusMetrics struct {
	registry    *prometheus.Registry
	invocations *prometheus.CounterVec
	duration    *prometheus.HistogramVec
}

// NewPrometheusMetrics creates and registers Prometheus metrics with a custom registry to avoid test conflicts.
func NewPrometheusMetrics() *PrometheusMetrics {
	reg := prometheus.NewRegistry()

	invocations := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mini_lambda_invocations_total",
			Help: "Total number of function invocations partitioned by function and status",
		},
		[]string{"function", "status"},
	)

	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mini_lambda_invocation_duration_seconds",
			Help:    "Execution duration of function invocations in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"function"},
	)

	reg.MustRegister(invocations)
	reg.MustRegister(duration)

	return &PrometheusMetrics{
		registry:    reg,
		invocations: invocations,
		duration:    duration,
	}
}

// RecordInvocation records invocation metrics into the Prometheus collectors.
func (p *PrometheusMetrics) RecordInvocation(functionName string, status string, durationMs int64) {
	p.invocations.WithLabelValues(functionName, status).Inc()
	p.duration.WithLabelValues(functionName).Observe(float64(durationMs) / 1000.0)
}

// Handler returns an HTTP handler exposing the Prometheus exposition format.
func (p *PrometheusMetrics) Handler() http.Handler {
	return promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{})
}

// NoopMetrics provides a zero-allocation mock implementation of MetricsRecorder.
type NoopMetrics struct{}

// NewNoopMetrics initializes a new NoopMetrics instance.
func NewNoopMetrics() *NoopMetrics {
	return &NoopMetrics{}
}

// RecordInvocation implements MetricsRecorder by performing a no-op.
func (n *NoopMetrics) RecordInvocation(_, _ string, _ int64) {}

// Handler returns an empty HTTP handler.
func (n *NoopMetrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
