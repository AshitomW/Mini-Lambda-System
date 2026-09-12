package runner_test

import (
	"context"
	"testing"
	"time"

	"AshitomW/mini-lambda/internal/domain"
	"AshitomW/mini-lambda/internal/runner"
)

func TestKubernetesRunnerInvoke(t *testing.T) {
	simClient := runner.NewSimulatedKubeClient()
	k8sRunner := runner.NewKubernetesRunner(simClient, "mini-lambda")

	ctx := context.Background()
	fn := domain.Function{
		ID:       "fn-k8s-1",
		Name:     "k8s-hello",
		Image:    "python:alpine",
		MemoryMB: 512,
		Env: map[string]string{
			"CLUSTER_NAME": "test-cluster",
		},
	}

	payload := []byte(`{"ping":"k8s"}`)
	invCtx := domain.InvocationContext{
		InvocationID: "inv-k8s-100",
		TraceID:      "trace-k8s-xyz",
	}

	res, err := k8sRunner.Invoke(ctx, fn, payload, invCtx, 5*time.Second)
	if err != nil {
		t.Fatalf("KubernetesRunner.Invoke failed: %v", err)
	}

	if res.Output != `processed: {"ping":"k8s"}` {
		t.Errorf("unexpected output: %s", res.Output)
	}

	if simClient.CreatedCount != 1 {
		t.Errorf("expected 1 pod created, got %d", simClient.CreatedCount)
	}
	if simClient.DeletedCount != 1 {
		t.Errorf("expected 1 pod cleaned up, got %d", simClient.DeletedCount)
	}

	imgs, err := k8sRunner.ListImages(ctx)
	if err != nil || len(imgs) == 0 {
		t.Errorf("expected images returned from k8s runner")
	}
}
