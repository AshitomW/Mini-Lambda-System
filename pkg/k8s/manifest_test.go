package k8s_test

import (
	"strings"
	"testing"

	"AshitomW/mini-lambda/internal/domain"
	"AshitomW/mini-lambda/pkg/k8s"
	"github.com/goccy/go-yaml"
)

func TestGenerateCRDManifest(t *testing.T) {
	fn := domain.Function{
		ID:         "fn-12345",
		Name:       "order_processor",
		Image:      "orders:v1.2",
		MemoryMB:   512,
		TimeoutSec: 60,
		Env: map[string]string{
			"LOG_LEVEL": "debug",
		},
	}

	crd, err := k8s.GenerateCRDManifest(fn, "serverless-prod")
	if err != nil {
		t.Fatalf("GenerateCRDManifest failed: %v", err)
	}

	if !strings.Contains(crd, "kind: Function") {
		t.Errorf("expected kind: Function in CRD")
	}
	if !strings.Contains(crd, "name: order-processor") {
		t.Errorf("expected sanitized name order-processor")
	}

	// Verify that the generated YAML unmarshals back into a typed struct cleanly!
	var parsed k8s.FunctionCRDManifest
	if err := yaml.Unmarshal([]byte(crd), &parsed); err != nil {
		t.Fatalf("failed to unmarshal generated YAML: %v", err)
	}

	if parsed.Metadata.Namespace != "serverless-prod" {
		t.Errorf("expected namespace serverless-prod, got %s", parsed.Metadata.Namespace)
	}
	if parsed.Spec.MemoryLimitMB != 512 {
		t.Errorf("expected MemoryLimitMB 512, got %d", parsed.Spec.MemoryLimitMB)
	}
	if len(parsed.Spec.Env) != 1 || parsed.Spec.Env[0].Name != "LOG_LEVEL" {
		t.Errorf("expected env var LOG_LEVEL preserved, got %+v", parsed.Spec.Env)
	}
}

func TestGenerateJobAndPodManifest(t *testing.T) {
	fn := domain.Function{
		ID:    "fn-999",
		Name:  "data-cruncher",
		Image: "cruncher:latest",
		Env: map[string]string{
			"WORKER_COUNT": "4",
		},
	}

	job, err := k8s.GenerateJobManifest(fn, "")
	if err != nil {
		t.Fatalf("GenerateJobManifest failed: %v", err)
	}

	var parsedJob k8s.JobManifest
	if err := yaml.Unmarshal([]byte(job), &parsedJob); err != nil {
		t.Fatalf("failed to unmarshal Job YAML: %v", err)
	}
	if parsedJob.Kind != "Job" {
		t.Errorf("expected kind Job, got %s", parsedJob.Kind)
	}
	if parsedJob.Metadata.Namespace != "mini-lambda" {
		t.Errorf("expected default namespace mini-lambda, got %s", parsedJob.Metadata.Namespace)
	}

	pod, err := k8s.GeneratePodManifest(fn, "")
	if err != nil {
		t.Fatalf("GeneratePodManifest failed: %v", err)
	}

	var parsedPod k8s.PodManifest
	if err := yaml.Unmarshal([]byte(pod), &parsedPod); err != nil {
		t.Fatalf("failed to unmarshal Pod YAML: %v", err)
	}
	if parsedPod.Kind != "Pod" {
		t.Errorf("expected kind Pod, got %s", parsedPod.Kind)
	}
	if !parsedPod.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem {
		t.Errorf("expected ReadOnlyRootFilesystem in pod security context")
	}
}
