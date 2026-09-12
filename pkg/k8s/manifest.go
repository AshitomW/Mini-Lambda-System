// Package k8s provides declarative, strongly-typed Kubernetes Custom Resource Definitions (CRD), Job, and Pod manifest generation.
package k8s

import (
	"fmt"
	"strings"

	"AshitomW/mini-lambda/internal/domain"
	"github.com/goccy/go-yaml"
)

// EnvVar represents an environment variable definition in a Kubernetes container.
type EnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// ResourceLimits represents compute resource allocations for a container.
type ResourceLimits struct {
	Memory string `yaml:"memory"`
	CPU    string `yaml:"cpu"`
}

// ResourceRequirements describes the compute resource requirements for a container.
type ResourceRequirements struct {
	Limits   ResourceLimits `yaml:"limits"`
	Requests ResourceLimits `yaml:"requests,omitempty"`
}

// SecurityContext holds container security and isolation parameters.
type SecurityContext struct {
	AllowPrivilegeEscalation bool     `yaml:"allowPrivilegeEscalation"`
	ReadOnlyRootFilesystem   bool     `yaml:"readOnlyRootFilesystem"`
	RunAsNonRoot             bool     `yaml:"runAsNonRoot,omitempty"`
	DropCapabilities         []string `yaml:"drop,omitempty"`
}

// Container represents a container specification in a Kubernetes Pod.
type Container struct {
	Name            string               `yaml:"name"`
	Image           string               `yaml:"image"`
	Resources       ResourceRequirements `yaml:"resources"`
	SecurityContext SecurityContext      `yaml:"securityContext"`
	Env             []EnvVar             `yaml:"env,omitempty"`
}

// ObjectMeta provides standard metadata for Kubernetes resources.
type ObjectMeta struct {
	Name         string            `yaml:"name,omitempty"`
	GenerateName string            `yaml:"generateName,omitempty"`
	Namespace    string            `yaml:"namespace"`
	Labels       map[string]string `yaml:"labels,omitempty"`
}

// PodSpec is the specification of the desired execution behavior of a Pod.
type PodSpec struct {
	RestartPolicy string      `yaml:"restartPolicy"`
	Containers    []Container `yaml:"containers"`
}

// PodManifest represents a Kubernetes v1 Pod manifest.
type PodManifest struct {
	APIVersion string     `yaml:"apiVersion"`
	Kind       string     `yaml:"kind"`
	Metadata   ObjectMeta `yaml:"metadata"`
	Spec       PodSpec    `yaml:"spec"`
}

// JobTemplateSpec describes the Pod template inside a Job.
type JobTemplateSpec struct {
	Metadata ObjectMeta `yaml:"metadata"`
	Spec     PodSpec    `yaml:"spec"`
}

// JobSpec describes the desired state of a batch/v1 Job.
type JobSpec struct {
	BackoffLimit          int             `yaml:"backoffLimit"`
	ActiveDeadlineSeconds int             `yaml:"activeDeadlineSeconds"`
	Template              JobTemplateSpec `yaml:"template"`
}

// JobManifest represents a Kubernetes batch/v1 Job manifest.
type JobManifest struct {
	APIVersion string     `yaml:"apiVersion"`
	Kind       string     `yaml:"kind"`
	Metadata   ObjectMeta `yaml:"metadata"`
	Spec       JobSpec    `yaml:"spec"`
}

// FunctionCRDSpec specifies the custom configuration for a mini-lambda.io/v1alpha1 Function.
type FunctionCRDSpec struct {
	Image         string   `yaml:"image"`
	MemoryLimitMB int64    `yaml:"memoryLimitMb"`
	TimeoutSec    int      `yaml:"timeoutSec"`
	Env           []EnvVar `yaml:"env,omitempty"`
}

// FunctionCRDManifest represents a declarative mini-lambda.io/v1alpha1 Function Custom Resource.
type FunctionCRDManifest struct {
	APIVersion string          `yaml:"apiVersion"`
	Kind       string          `yaml:"kind"`
	Metadata   ObjectMeta      `yaml:"metadata"`
	Spec       FunctionCRDSpec `yaml:"spec"`
}

// BuildCRDManifest creates a strongly-typed FunctionCRDManifest model.
func BuildCRDManifest(fn domain.Function, namespace string) FunctionCRDManifest {
	if namespace == "" {
		namespace = "mini-lambda"
	}
	memMB := fn.MemoryMB
	if memMB <= 0 {
		memMB = 256
	}
	timeoutSec := fn.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 120
	}

	return FunctionCRDManifest{
		APIVersion: "mini-lambda.io/v1alpha1",
		Kind:       "Function",
		Metadata: ObjectMeta{
			Name:      sanitizeKubeName(fn.Name),
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "mini-lambda",
				"mini-lambda.io/function-id":   fn.ID,
			},
		},
		Spec: FunctionCRDSpec{
			Image:         fn.Image,
			MemoryLimitMB: memMB,
			TimeoutSec:    timeoutSec,
			Env:           toKubeEnv(fn.Env),
		},
	}
}

// GenerateCRDManifest renders a mini-lambda.io/v1alpha1 declarative Custom Resource manifest as YAML.
func GenerateCRDManifest(fn domain.Function, namespace string) (string, error) {
	manifest := BuildCRDManifest(fn, namespace)
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal CRD manifest: %w", err)
	}
	return string(data), nil
}

// BuildJobManifest creates a strongly-typed JobManifest model for isolated batch function execution.
func BuildJobManifest(fn domain.Function, namespace string) JobManifest {
	if namespace == "" {
		namespace = "mini-lambda"
	}
	memMB := fn.MemoryMB
	if memMB <= 0 {
		memMB = 256
	}
	timeoutSec := fn.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 120
	}

	safeName := sanitizeKubeName(fn.Name)

	return JobManifest{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Metadata: ObjectMeta{
			GenerateName: fmt.Sprintf("minilambda-%s-", safeName),
			Namespace:    namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "mini-lambda",
				"mini-lambda.io/function":      safeName,
			},
		},
		Spec: JobSpec{
			BackoffLimit:          0,
			ActiveDeadlineSeconds: timeoutSec,
			Template: JobTemplateSpec{
				Metadata: ObjectMeta{
					Labels: map[string]string{
						"mini-lambda.io/function": safeName,
					},
				},
				Spec: PodSpec{
					RestartPolicy: "Never",
					Containers: []Container{
						{
							Name:  "function",
							Image: fn.Image,
							Resources: ResourceRequirements{
								Limits: ResourceLimits{
									Memory: fmt.Sprintf("%dMi", memMB),
									CPU:    "1000m",
								},
								Requests: ResourceLimits{
									Memory: fmt.Sprintf("%dMi", memMB/2),
									CPU:    "100m",
								},
							},
							SecurityContext: SecurityContext{
								AllowPrivilegeEscalation: false,
								ReadOnlyRootFilesystem:   true,
								RunAsNonRoot:             true,
								DropCapabilities:         []string{"ALL"},
							},
							Env: toKubeEnv(fn.Env),
						},
					},
				},
			},
		},
	}
}

// GenerateJobManifest renders a Kubernetes batch/v1 Job manifest as YAML.
func GenerateJobManifest(fn domain.Function, namespace string) (string, error) {
	manifest := BuildJobManifest(fn, namespace)
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Job manifest: %w", err)
	}
	return string(data), nil
}

// BuildPodManifest creates a strongly-typed PodManifest model for synchronous container execution.
func BuildPodManifest(fn domain.Function, namespace string) PodManifest {
	if namespace == "" {
		namespace = "mini-lambda"
	}
	memMB := fn.MemoryMB
	if memMB <= 0 {
		memMB = 256
	}

	safeName := sanitizeKubeName(fn.Name)

	return PodManifest{
		APIVersion: "v1",
		Kind:       "Pod",
		Metadata: ObjectMeta{
			GenerateName: fmt.Sprintf("pod-%s-", safeName),
			Namespace:    namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "mini-lambda",
				"mini-lambda.io/function":      safeName,
			},
		},
		Spec: PodSpec{
			RestartPolicy: "Never",
			Containers: []Container{
				{
					Name:  "function-runner",
					Image: fn.Image,
					Resources: ResourceRequirements{
						Limits: ResourceLimits{
							Memory: fmt.Sprintf("%dMi", memMB),
							CPU:    "1000m",
						},
					},
					SecurityContext: SecurityContext{
						AllowPrivilegeEscalation: false,
						ReadOnlyRootFilesystem:   true,
						RunAsNonRoot:             true,
						DropCapabilities:         []string{"ALL"},
					},
					Env: toKubeEnv(fn.Env),
				},
			},
		},
	}
}

// GeneratePodManifest renders an ephemeral Kubernetes v1 Pod manifest as YAML.
func GeneratePodManifest(fn domain.Function, namespace string) (string, error) {
	manifest := BuildPodManifest(fn, namespace)
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Pod manifest: %w", err)
	}
	return string(data), nil
}

func toKubeEnv(env map[string]string) []EnvVar {
	if len(env) == 0 {
		return nil
	}
	vars := make([]EnvVar, 0, len(env))
	for k, v := range env {
		vars = append(vars, EnvVar{
			Name:  k,
			Value: v,
		})
	}
	return vars
}

func sanitizeKubeName(name string) string {
	res := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return '-'
	}, name)
	res = strings.Trim(res, "-")
	if res == "" {
		return "function"
	}
	return res
}
