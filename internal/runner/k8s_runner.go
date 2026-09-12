// Package runner provides Kubernetes execution infrastructure for serverless functions.
package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"AshitomW/mini-lambda/internal/domain"
)

// KubePodSpec represents the minimal Pod configuration needed to execute a serverless function.
type KubePodSpec struct {
	Name         string
	Namespace    string
	Image        string
	Env          []string
	Payload      []byte
	MemoryMB     int64
	CPUShares    int64
	SecurityOpts KubeSecurityOptions
}

// KubeSecurityOptions defines pod security constraints.
type KubeSecurityOptions struct {
	ReadOnlyRootFilesystem   bool
	AllowPrivilegeEscalation bool
	RunAsNonRoot             bool
	DropCapabilities         []string
}

// KubePodStatus represents the status of an executed pod.
type KubePodStatus struct {
	Phase      string
	ExitCode   int
	DurationMs int64
}

// KubePodClient specifies the contract for interacting with Kubernetes Pod resources.
type KubePodClient interface {
	CreatePod(ctx context.Context, spec KubePodSpec) (string, error)
	WaitPod(ctx context.Context, namespace, podName string, timeout time.Duration) (KubePodStatus, error)
	GetPodLogs(ctx context.Context, namespace, podName string) (string, string, error)
	DeletePod(ctx context.Context, namespace, podName string) error
	ListImages(ctx context.Context) ([]string, error)
}

// KubernetesRunner orchestrates function execution as ephemeral pods in a Kubernetes cluster.
type KubernetesRunner struct {
	client    KubePodClient
	namespace string
}

// NewKubernetesRunner constructs a new KubernetesRunner with the given client and target namespace.
func NewKubernetesRunner(client KubePodClient, namespace string) *KubernetesRunner {
	if namespace == "" {
		namespace = "mini-lambda"
	}
	return &KubernetesRunner{
		client:    client,
		namespace: namespace,
	}
}

// Invoke executes a function inside an isolated ephemeral Kubernetes Pod.
func (r *KubernetesRunner) Invoke(ctx context.Context, fn domain.Function, payload []byte, invCtx domain.InvocationContext, timeout time.Duration) (*domain.InvocationResult, error) {
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	podName := fmt.Sprintf("minilambda-%s-%d", sanitizeK8sName(fn.Name), time.Now().UnixNano()%1000000)

	memLimit := fn.MemoryMB
	if memLimit <= 0 {
		memLimit = 256
	}

	spec := KubePodSpec{
		Name:      podName,
		Namespace: r.namespace,
		Image:     fn.Image,
		Env:       invCtx.ToEnvSlice(),
		Payload:   payload,
		MemoryMB:  memLimit,
		SecurityOpts: KubeSecurityOptions{
			ReadOnlyRootFilesystem:   true,
			AllowPrivilegeEscalation: false,
			RunAsNonRoot:             true,
			DropCapabilities:         []string{"ALL"},
		},
	}

	createdPod, err := r.client.CreatePod(execCtx, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes pod: %w", err)
	}

	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = r.client.DeletePod(cleanupCtx, r.namespace, createdPod)
	}()

	status, err := r.client.WaitPod(execCtx, r.namespace, createdPod, timeout)
	if err != nil {
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			return nil, domain.ErrExecutionTimeout
		}
		return nil, fmt.Errorf("kubernetes pod execution failed: %w", err)
	}

	if status.ExitCode != 0 {
		return nil, fmt.Errorf("kubernetes pod exited with non-zero exit code: %d", status.ExitCode)
	}

	stdout, stderr, err := r.client.GetPodLogs(execCtx, r.namespace, createdPod)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve pod logs: %w", err)
	}

	duration := time.Since(start)
	combinedLogs := stdout
	if stderr != "" {
		if combinedLogs != "" {
			combinedLogs += "\n"
		}
		combinedLogs += stderr
	}

	return &domain.InvocationResult{
		Output:     stdout,
		Logs:       combinedLogs,
		DurationMs: duration.Milliseconds(),
	}, nil
}

// LoadImage simulates pre-loading or pulling an image into the cluster.
func (r *KubernetesRunner) LoadImage(_ context.Context, _ io.Reader) error {
	return nil
}

// ListImages returns the available container image tags.
func (r *KubernetesRunner) ListImages(ctx context.Context) ([]string, error) {
	return r.client.ListImages(ctx)
}

// Close terminates any active runner connections.
func (r *KubernetesRunner) Close() error {
	return nil
}

func sanitizeK8sName(name string) string {
	var clean []rune
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			clean = append(clean, r)
		} else {
			clean = append(clean, '-')
		}
	}
	res := string(clean)
	if res == "" {
		return "function"
	}
	return res
}

// SimulatedKubeClient is a thread-safe in-memory Kubernetes pod lifecycle simulator for integration testing.
type SimulatedKubeClient struct {
	mu           sync.Mutex
	pods         map[string]KubePodSpec
	ExecuteFunc  func(spec KubePodSpec) (stdout, stderr string, exitCode int, err error)
	Images       []string
	CreatedCount int
	DeletedCount int
}

// NewSimulatedKubeClient initializes a SimulatedKubeClient with sensible default execution behaviors.
func NewSimulatedKubeClient() *SimulatedKubeClient {
	return &SimulatedKubeClient{
		pods:   make(map[string]KubePodSpec),
		Images: []string{"alpine:latest", "python:3.9-alpine"},
		ExecuteFunc: func(spec KubePodSpec) (string, string, int, error) {
			output := "processed: " + string(spec.Payload)
			return output, "execution log", 0, nil
		},
	}
}

// CreatePod records pod creation.
func (c *SimulatedKubeClient) CreatePod(_ context.Context, spec KubePodSpec) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.CreatedCount++
	c.pods[spec.Name] = spec
	return spec.Name, nil
}

// WaitPod simulates waiting for pod execution completion.
func (c *SimulatedKubeClient) WaitPod(ctx context.Context, _, podName string, _ time.Duration) (KubePodStatus, error) {
	select {
	case <-ctx.Done():
		return KubePodStatus{Phase: "Failed"}, ctx.Err()
	default:
	}

	c.mu.Lock()
	_, ok := c.pods[podName]
	c.mu.Unlock()

	if !ok {
		return KubePodStatus{Phase: "NotFound"}, errors.New("pod not found")
	}

	return KubePodStatus{
		Phase:      "Succeeded",
		ExitCode:   0,
		DurationMs: 20,
	}, nil
}

// GetPodLogs returns simulated logs from the pod execution.
func (c *SimulatedKubeClient) GetPodLogs(_ context.Context, _, podName string) (string, string, error) {
	c.mu.Lock()
	spec, ok := c.pods[podName]
	c.mu.Unlock()

	if !ok {
		return "", "", errors.New("pod not found")
	}

	if c.ExecuteFunc != nil {
		stdout, stderr, _, err := c.ExecuteFunc(spec)
		return stdout, stderr, err
	}

	return string(spec.Payload), "logs", nil
}

// DeletePod records pod removal.
func (c *SimulatedKubeClient) DeletePod(_ context.Context, _, podName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.pods, podName)
	c.DeletedCount++
	return nil
}

// ListImages returns registered cluster images.
func (c *SimulatedKubeClient) ListImages(_ context.Context) ([]string, error) {
	return c.Images, nil
}
