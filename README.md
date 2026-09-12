# Mini-Lambda-System

[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/Tests-Passing%20with%20--race-success.svg)](https://github.com/AshitomW/Mini-Lambda-System)
[![CloudEvents](https://img.shields.io/badge/CloudEvents-v1.0-blueviolet.svg)](https://cloudevents.io)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-CRD%20%7C%20Job%20%7C%20Pod-326CE5?logo=kubernetes&logoColor=white)](https://kubernetes.io)

**Mini-Lambda-System** is a production-grade, lightweight serverless runtime and container orchestration engine built in idiomatic Go. Designed around clean architecture principles, it provides on-demand container sandboxing, sub-millisecond warm worker pooling, pluggable Kubernetes execution, zero-trust mutual TLS (mTLS), CNCF CloudEvents 1.0 delivery, and automated Dead Letter Queue (DLQ) resilience.

---

## Key Capabilities

- **Multi-Runtime Execution**:
  - **Docker Runner**: Direct local container isolation with stdout/stderr stream demultiplexing.
  - **Kubernetes Runner**: Ephemeral Pod and batch/v1 Job orchestration with automated lifecycle tracking.
  - **Standby Warm Pool**: Pre-warmed container worker pool dropping invocation latencies from ~1,200ms to ~15ms, with an automatic scale-to-zero idle reaper.
- **Zero-Trust Security & Hardening**:
  - **TLS 1.3 & mTLS**: Client certificate validation with identity extraction (`tls.RequireAndVerifyClientCert`).
  - **PKI Generator (`pkg/certutil`)**: In-memory X.509 Root CA, server, and client certificate suite for development and testing.
  - **Container Sandboxing**: Default `NetworkMode: "none"` (SSRF prevention), `CapDrop: ALL`, `no-new-privileges: true`, read-only root with non-exec tmpfs `/tmp`, and `PidsLimit: 100`.
  - **RBAC API Keys**: Role separation (`admin` vs `invoker`) enforced via constant-time HMAC comparison.
  - **Webhook HMAC-SHA256**: Signature verification (`X-Hub-Signature-256`) against per-function secrets.
  - **Secret Redaction**: Automated masking (`********`) of sensitive environment variables and credentials in API outputs.
- **Standardized Event Delivery**:
  - **CNCF CloudEvents 1.0**: Standard envelope validation and JSON serialization.
  - **Execution Context Injection**: Automatic passing of deadlines, trace IDs, caller identity, and environment variables into containers.
  - **Dual Identifier Resolution**: Transparent lookup by function name or UUID.
  - **Generic Webhook Gateway**: Ingestion via `POST /hooks/:identifier` for synchronous and asynchronous triggers.
- **Resilience & Observability**:
  - **Async Retry Engine**: Automated retries with exponential backoff (`time.Duration(1<<attempt) * 50ms`).
  - **Dead Letter Queue (DLQ)**: Quarantines failed messages with endpoints to inspect (`GET /invocations/dlq`) and replay (`POST /invocations/:id/retry`).
  - **Distributed Tracing**: W3C `traceparent`, `X-Request-ID`, and `X-Trace-ID` propagation.
  - **Prometheus Metrics**: Real-time counters and latency duration histograms at `/metrics`.
- **Declarative Kubernetes Generator (`pkg/k8s`)**:
  - Strongly-typed Go structs serialized with `goccy/go-yaml` (zero manual string concatenation) generating Custom Resource Definitions (CRDs), Jobs, and Pod manifests.
- **Official Go Client SDK (`pkg/client`)**:
  - Idiomatic, thread-safe SDK supporting sync/async calls, CloudEvents, webhooks, polling, mTLS, API keys, and DLQ management.

---

## Architecture Overview

```
Mini-Lambda-System/
├── cmd/
│   └── server/               # Application binary entrypoint
├── internal/
│   ├── app/                  # Application lifecycle, TLS 1.3/mTLS configuration & graceful shutdown
│   ├── config/               # Environment-based configuration with safe defaults
│   ├── domain/               # Domain entities (Function, Invocation, CloudEvent, InvocationContext)
│   ├── handler/              # Gin HTTP handlers, DTOs, routing, and security middleware
│   ├── metrics/              # Prometheus metrics collector (counters, histograms)
│   ├── repository/           # Thread-safe storage (atomic file persistence, in-memory DLQ)
│   ├── runner/               # Execution backends:
│   │   ├── docker_runner.go  # Hardened Docker runner with stdcopy demuxing
│   │   ├── k8s_runner.go     # Kubernetes ephemeral Pod runner
│   │   ├── warm_pool.go      # Standby warm container pool with scale-to-zero reaper
│   │   └── mock_runner.go    # Thread-safe mock runner for tests
│   └── service/              # Core domain services (Functions, Invocations, Images)
├── pkg/
│   ├── certutil/             # Dynamic X.509 PKI generator (CA, Server, Client)
│   ├── client/               # Official Go Client SDK
│   └── k8s/                  # Strongly-typed declarative Kubernetes manifest generator
├── function/                 # Sample function definitions & assets
└── main.go                   # Root entrypoint
```

---

## Quick Start

### 1. Prerequisites

- **Go**: Version 1.24 or higher
- **Docker Engine**: Running locally

### 2. Installation & Running

```bash
# Clone the repository
git clone https://github.com/AshitomW/Mini-Lambda-System.git
cd Mini-Lambda-System

# Download and verify dependencies
go mod tidy

# Run all tests with the Go race detector
go test -v -race ./...

# Start the Mini-Lambda server
go run .
```

The server starts on port `8300` (or `PORT` environment variable).

---

## Configuration Reference

The application is configured via environment variables:

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PORT` | `8300` | HTTP listening port |
| `DATA_DIR` | `function` | Directory path for atomic function persistence |
| `MAX_CONCURRENT_INVOCATIONS` | `20` | Concurrency semaphore capacity limit |
| `DEFAULT_TIMEOUT_SEC` | `120` | Default execution timeout in seconds |
| `MAX_TIMEOUT_SEC` | `900` | Maximum allowable execution timeout |
| `SHUTDOWN_TIMEOUT_SEC` | `10` | Graceful shutdown drain deadline |
| `CONTAINER_MEMORY_LIMIT_MB` | `256` | Default container RAM allocation |
| `CONTAINER_CPU_SHARES` | `1024` | Default container CPU shares |
| `TLS_ENABLED` | `false` | Enable TLS 1.3 listener |
| `TLS_CERT_FILE` | `""` | Path to TLS server certificate PEM |
| `TLS_KEY_FILE` | `""` | Path to TLS server private key PEM |
| `MTLS_ENABLED` | `false` | Enforce mutual TLS client certificate verification |
| `MTLS_CLIENT_CA_FILE` | `""` | Path to trusted Root CA certificate PEM |
| `AUTH_ENABLED` | `false` | Enable API Key / Bearer token authorization |
| `ADMIN_API_KEY` | `""` | Key granting full system administrative access |
| `INVOKER_API_KEY` | `""` | Key granting function invocation permissions |
| `CONTAINER_NETWORK_ENABLED` | `false` | Default container bridge network policy |
| `MAX_RETRIES` | `3` | Maximum async retries before DLQ routing |
| `WARM_POOL_ENABLED` | `true` | Enable standby warm execution pool |
| `WARM_POOL_IDLE_TIMEOUT_SEC` | `60` | Warm worker idle timeout before scale-to-zero |

---

## API Reference

### Function Management

#### Register Function
`POST /functions`
```json
{
  "name": "image-processor",
  "image": "python:3.11-alpine",
  "env": {
    "LOG_LEVEL": "INFO",
    "API_KEY": "secret-key-to-mask"
  },
  "allow_network": false,
  "webhook_secret": "my-webhook-secret",
  "memory_mb": 512,
  "timeout_sec": 60
}
```
*Sensitive environment variables (`KEY`, `SECRET`, `TOKEN`, `PASSWORD`, `AUTH`) are automatically redacted in responses.*

#### List Functions
`GET /functions`

#### Get Function Details
`GET /functions/:id` *(accepts UUID or function name)*

#### Export Declarative Kubernetes YAML
`GET /functions/:id/k8s-manifest?format=crd|job|pod&namespace=mini-lambda`
Returns declarative Kubernetes YAML specifications generated via strongly-typed Go structs.

---

### Invocations

#### Synchronous Invocation
`POST /invoke/:identifier`
```json
{
  "event": {
    "action": "resize",
    "width": 800
  },
  "timeout": 30
}
```

#### CNCF CloudEvent Invocation
`POST /invoke/:identifier`
```json
{
  "cloud_event": {
    "specversion": "1.0",
    "id": "event-uuid-001",
    "source": "https://api.myapp.com/orders",
    "type": "order.created",
    "time": "2026-09-12T12:00:00Z",
    "datacontenttype": "application/json",
    "data": {
      "order_id": "ORD-12345"
    }
  }
}
```

#### Asynchronous Invocation
`POST /invoke/:identifier/async`
Returns `HTTP 202 Accepted` with invocation ID and status `PENDING`. Automatically triggers retry backoff and DLQ on failure.

#### Webhook Ingestion
`POST /hooks/:identifier` *(optional `?async=true`)*
Accepts direct payloads and validates HMAC signatures via `X-Hub-Signature-256` or `X-Signature-SHA256` if `webhook_secret` is configured.

---

### Resilience & Dead Letter Queue (DLQ)

#### Query Async Invocation Status
`GET /invocations/:invocation_id`

#### List Invocations in Dead Letter Queue
`GET /invocations/dlq`
Returns all failed async invocations that have exhausted their retry limit.

#### Replay Dead-Lettered Invocation
`POST /invocations/:invocation_id/retry`
Resets retry count, clears error state, and re-queues the message for execution.

---

### System & Telemetry

- **Health Check**: `GET /health` (`{"status": "UP"}`)
- **Prometheus Metrics**: `GET /metrics`
- **Docker Image Upload**: `POST /images` *(multipart file)*
- **List Docker Images**: `GET /images`

---

## Go Client SDK (`pkg/client`)

Mini-Lambda provides an official, idiomatic Go SDK for integrating into applications.

### SDK Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"AshitomW/mini-lambda/pkg/client"
)

func main() {
	ctx := context.Background()

	// Initialize SDK client with optional API key or mTLS
	sdk := client.NewClient("http://localhost:8300",
		client.WithAPIKey("my-api-key"),
	)

	// 1. Check service health
	if ok, _ := sdk.HealthCheck(ctx); !ok {
		log.Fatal("Service unreachable")
	}

	// 2. Synchronous Invocation
	res, err := sdk.InvokeSync(ctx, "hello-python", map[string]string{
		"message": "Hello from Go SDK!",
	}, client.InvokeOptions{
		Timeout: 10 * time.Second,
		TraceID: "trace-abc-123",
	})
	if err != nil {
		log.Fatalf("Sync invoke failed: %v", err)
	}
	fmt.Printf("Output: %s (took %d ms)\n", res.Result, res.DurationMs)

	// 3. Asynchronous Invocation with Polling
	asyncRes, err := sdk.InvokeAsync(ctx, "hello-python", map[string]string{
		"task": "background-process",
	}, client.InvokeOptions{})
	if err != nil {
		log.Fatalf("Async invoke failed: %v", err)
	}

	// Wait for async result with automatic polling
	finalInv, err := sdk.WaitForAsyncResult(ctx, asyncRes.InvocationID, 500*time.Millisecond)
	if err != nil {
		log.Fatalf("Async execution failed: %v", err)
	}
	fmt.Printf("Async Completed with Status: %s\n", finalInv.Status)

	// 4. Dead Letter Queue Management
	dlq, err := sdk.ListDeadLetterInvocations(ctx)
	if err == nil && len(dlq) > 0 {
		fmt.Printf("Found %d dead-lettered invocations. Replaying first...\n", len(dlq))
		_, _ = sdk.RetryDeadLetter(ctx, dlq[0].ID)
	}
}
```

---

## Security Model

```
       [Client Traffic]
              │
              ▼
   ┌──────────────────────┐
   │ TLS 1.3 / mTLS Check │  (Client Cert Verification against Root CA)
   └──────────┬───────────┘
              ▼
   ┌──────────────────────┐
   │  Tracing Middleware  │  (X-Request-ID, W3C traceparent injection)
   └──────────┬───────────┘
              ▼
   ┌──────────────────────┐
   │   RBAC Auth Check    │  (Constant-time HMAC comparison: Admin vs Invoker)
   └──────────┬───────────┘
              ▼
   ┌──────────────────────┐
   │ Webhook HMAC Check   │  (SHA256 signature verification per function)
   └──────────┬───────────┘
              ▼
   ┌────────────────────────────────────────┐
   │       Hardened Container Sandbox       │
   │  • NetworkMode: none (SSRF protection) │
   │  • CapDrop: ALL                        │
   │  • no-new-privileges: true             │
   │  • Tmpfs /tmp: rw,noexec,nosuid,size=64m│
   │  • PidsLimit: 100                      │
   └────────────────────────────────────────┘
```

---

## Testing & Quality Assurance

The codebase adheres to idiomatic Go standards, maintains zero data races, and includes comprehensive test suites across all packages:

```bash
# Run all tests with race detector
go test -v -race ./...
```

| Package | Test Coverage Summary |
| :--- | :--- |
| `internal/config` | Environment overrides, type parsing, safe default validations |
| `internal/domain` | Validation rules, secret sanitization, context mapping, CloudEvents 1.0 |
| `internal/repository` | Thread-safe in-memory DLQ, atomic file persistence, corrupt JSON recovery |
| `internal/runner` | Ephemeral K8s pod lifecycle, stream demuxing, warm pool leasing & reaper |
| `internal/service` | Bounded concurrency semaphore, exponential backoff retries, DLQ replay |
| `internal/handler` | All HTTP endpoints, tracing, RBAC auth, webhook HMAC verification |
| `pkg/certutil` | In-memory RSA-2048 Root CA, server cert, and client cert generation |
| `pkg/k8s` | Strongly-typed YAML generation for CRDs, Jobs, and Pod manifests |
| `pkg/client` | SDK sync, async, CloudEvents, webhooks, polling, DLQ, and mTLS transport |

---

## License

This project is licensed under the MIT License.
