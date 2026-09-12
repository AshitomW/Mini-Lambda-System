// Package client provides an idiomatic Go SDK for interacting with the Mini-Lambda-System.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"AshitomW/mini-lambda/internal/domain"
)

var (
	// ErrInvocationPending indicates an async invocation has not yet finished.
	ErrInvocationPending = errors.New("invocation still running or pending")
	// ErrInvocationFailed indicates an async invocation completed with an error.
	ErrInvocationFailed = errors.New("invocation failed")
)

// Client is an HTTP client for communicating with Mini-Lambda.
type Client struct {
	baseURL    string
	httpClient *http.Client
	headers    http.Header
}

// Option configures Client settings.
type Option func(*Client)

// WithHTTPClient allows passing a customized *http.Client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithCustomHeader adds a global default header to all outbound requests.
func WithCustomHeader(key, value string) Option {
	return func(c *Client) {
		c.headers.Set(key, value)
	}
}

// NewClient initializes a new Mini-Lambda Go SDK client.
func NewClient(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		headers: make(http.Header),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// InvokeOptions configures a specific function execution request.
type InvokeOptions struct {
	Timeout   time.Duration
	TraceID   string
	RequestID string
	Headers   map[string]string
}

// SyncInvokeResult encapsulates synchronous execution results.
type SyncInvokeResult struct {
	Result       string    `json:"result"`
	Logs         string    `json:"logs,omitempty"`
	DurationMs   int64     `json:"duration"`
	InvocationID string    `json:"invocation_id,omitempty"`
	TraceID      string    `json:"trace_id,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// AsyncInvokeResult encapsulates accepted asynchronous invocation metadata.
type AsyncInvokeResult struct {
	InvocationID string    `json:"invocation_id"`
	Status       string    `json:"status"`
	TraceID      string    `json:"trace_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// WebhookResult encapsulates webhook trigger execution output.
type WebhookResult struct {
	Status       string `json:"status"`
	Function     string `json:"function"`
	InvocationID string `json:"invocation_id,omitempty"`
	Result       string `json:"result,omitempty"`
	DurationMs   int64  `json:"duration_ms,omitempty"`
}

// RegisterFunction registers a new function with optional environment variables and memory limits.
func (c *Client) RegisterFunction(ctx context.Context, fn domain.Function) (domain.Function, error) {
	body, err := json.Marshal(fn)
	if err != nil {
		return domain.Function{}, fmt.Errorf("failed to marshal function: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/functions", bytes.NewReader(body))
	if err != nil {
		return domain.Function{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.Function{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return domain.Function{}, c.parseError(resp)
	}

	var created domain.Function
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return domain.Function{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return created, nil
}

// GetFunction retrieves a function definition by name or UUID.
func (c *Client) GetFunction(ctx context.Context, identifier string) (domain.Function, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/functions/"+identifier, nil)
	if err != nil {
		return domain.Function{}, err
	}
	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.Function{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return domain.Function{}, c.parseError(resp)
	}

	var fn domain.Function
	if err := json.NewDecoder(resp.Body).Decode(&fn); err != nil {
		return domain.Function{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return fn, nil
}

// ListFunctions returns all registered functions.
func (c *Client) ListFunctions(ctx context.Context) ([]domain.Function, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/functions", nil)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var list []domain.Function
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return list, nil
}

// InvokeSync executes a function synchronously.
func (c *Client) InvokeSync(ctx context.Context, identifier string, payload any, opts ...InvokeOptions) (*SyncInvokeResult, error) {
	var opt InvokeOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	reqBody := map[string]any{
		"event":   payload,
		"timeout": int(opt.Timeout.Seconds()),
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal invoke request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/invoke/"+identifier, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyHeaders(req)
	c.applyInvokeOptions(req, opt)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result SyncInvokeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode sync response: %w", err)
	}

	return &result, nil
}

// InvokeCloudEvent sends a standardized CNCF CloudEvent 1.0 envelope to the target function.
func (c *Client) InvokeCloudEvent(ctx context.Context, identifier string, ce domain.CloudEvent, opts ...InvokeOptions) (*SyncInvokeResult, error) {
	if err := ce.Validate(); err != nil {
		return nil, fmt.Errorf("invalid cloud event: %w", err)
	}

	var opt InvokeOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	reqBody := map[string]any{
		"cloud_event": ce,
		"timeout":     int(opt.Timeout.Seconds()),
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cloud event request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/invoke/"+identifier, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyHeaders(req)
	c.applyInvokeOptions(req, opt)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result SyncInvokeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode sync response: %w", err)
	}

	return &result, nil
}

// InvokeAsync dispatches execution to a background worker and returns immediate acceptance info.
func (c *Client) InvokeAsync(ctx context.Context, identifier string, payload any, opts ...InvokeOptions) (*AsyncInvokeResult, error) {
	var opt InvokeOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	reqBody := map[string]any{
		"event":   payload,
		"timeout": int(opt.Timeout.Seconds()),
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal async request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/invoke/"+identifier+"/async", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyHeaders(req)
	c.applyInvokeOptions(req, opt)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return nil, c.parseError(resp)
	}

	var result AsyncInvokeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode async response: %w", err)
	}

	return &result, nil
}

// GetInvocation queries the current state and result of an async execution.
func (c *Client) GetInvocation(ctx context.Context, invocationID string) (*domain.AsyncInvocation, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/invocations/"+invocationID, nil)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var inv domain.AsyncInvocation
	if err := json.NewDecoder(resp.Body).Decode(&inv); err != nil {
		return nil, fmt.Errorf("failed to decode invocation: %w", err)
	}

	return &inv, nil
}

// WaitForAsyncResult polls the status of an async invocation until completion, timeout, or failure.
func (c *Client) WaitForAsyncResult(ctx context.Context, invocationID string, pollInterval time.Duration) (*domain.AsyncInvocation, error) {
	if pollInterval <= 0 {
		pollInterval = 100 * time.Millisecond
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			inv, err := c.GetInvocation(ctx, invocationID)
			if err != nil {
				return nil, err
			}

			switch inv.Status {
			case domain.StatusCompleted:
				return inv, nil
			case domain.StatusFailed:
				return inv, fmt.Errorf("%w: %s", ErrInvocationFailed, inv.Error)
			case domain.StatusPending, domain.StatusRunning:
				// continue polling
			default:
				return inv, fmt.Errorf("unknown invocation status: %s", inv.Status)
			}
		}
	}
}

// SendWebhook delivers a raw webhook payload directly to the /hooks/:identifier endpoint.
func (c *Client) SendWebhook(ctx context.Context, identifier string, body []byte, isAsync bool, headers map[string]string) (*WebhookResult, error) {
	url := c.baseURL + "/hooks/" + identifier
	if isAsync {
		url += "?async=true"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyHeaders(req)

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, c.parseError(resp)
	}

	var result WebhookResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode webhook response: %w", err)
	}

	return &result, nil
}

// HealthCheck checks whether the Mini-Lambda service is reachable and UP.
func (c *Client) HealthCheck(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return false, err
	}
	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

func (c *Client) applyHeaders(req *http.Request) {
	for k, vals := range c.headers {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
}

func (c *Client) applyInvokeOptions(req *http.Request, opt InvokeOptions) {
	if opt.TraceID != "" {
		req.Header.Set("traceparent", opt.TraceID)
	}
	if opt.RequestID != "" {
		req.Header.Set("X-Request-ID", opt.RequestID)
	}
	for k, v := range opt.Headers {
		req.Header.Set(k, v)
	}
}

func (c *Client) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, errResp.Error)
	}
	return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
}
