package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"AshitomW/mini-lambda/internal/config"
	"AshitomW/mini-lambda/internal/domain"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// DockerRunner executes function containers against a local or remote Docker daemon.
type DockerRunner struct {
	client *client.Client
	cfg    config.Config
}

// NewDockerRunner initializes a new DockerRunner client.
func NewDockerRunner(cfg config.Config) (*DockerRunner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrRunnerUnavailable, err)
	}

	return &DockerRunner{
		client: cli,
		cfg:    cfg,
	}, nil
}

// Invoke executes a function inside an isolated Docker container with resource boundaries and timeout control.
func (r *DockerRunner) Invoke(ctx context.Context, fn domain.Function, payload []byte, timeout time.Duration) (*domain.InvocationResult, error) {
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	resp, err := r.client.ContainerCreate(execCtx, &container.Config{
		Image:        fn.Image,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    true,
		StdinOnce:    true,
	}, &container.HostConfig{
		Resources: container.Resources{
			Memory:   r.cfg.MemoryLimitBytes,
			CPUShares: r.cfg.CPUShares,
		},
	}, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("container creation failed: %w", err)
	}

	containerID := resp.ID
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = r.client.ContainerRemove(cleanupCtx, containerID, client.ContainerRemoveOptions{Force: true})
	}()

	if err := r.client.ContainerStart(execCtx, containerID, client.ContainerStartOptions{}); err != nil {
		return nil, fmt.Errorf("container start failed: %w", err)
	}

	hijack, err := r.client.ContainerAttach(execCtx, containerID, client.ContainerAttachOptions{
		Stdin:  true,
		Stdout: true,
		Stderr: true,
		Stream: true,
	})
	if err != nil {
		return nil, fmt.Errorf("container attach failed: %w", err)
	}
	defer hijack.Close()

	if len(payload) > 0 {
		if _, err := hijack.Conn.Write(payload); err != nil {
			return nil, fmt.Errorf("failed to write payload to container: %w", err)
		}
	}
	_ = hijack.CloseWrite()

	statusCh, errCh := r.client.ContainerWait(execCtx, containerID, container.WaitConditionNextExit)

	select {
	case <-execCtx.Done():
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		_ = r.client.ContainerStop(stopCtx, containerID, client.ContainerStopOptions{})
		return nil, domain.ErrExecutionTimeout

	case err := <-errCh:
		if err != nil {
			return nil, fmt.Errorf("container execution error: %w", err)
		}
	case <-statusCh:
	}

	logCtx, logCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer logCancel()

	logsReader, err := r.client.ContainerLogs(logCtx, containerID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read container logs: %w", err)
	}
	defer logsReader.Close()

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	if err := DemuxStream(logsReader, &stdoutBuf, &stderrBuf); err != nil {
		return nil, fmt.Errorf("failed to demux container streams: %w", err)
	}

	duration := time.Since(start)

	combinedLogs := stdoutBuf.String()
	if stderrBuf.Len() > 0 {
		if combinedLogs != "" {
			combinedLogs += "\n"
		}
		combinedLogs += stderrBuf.String()
	}

	return &domain.InvocationResult{
		Output:     stdoutBuf.String(),
		Logs:       combinedLogs,
		DurationMs: duration.Milliseconds(),
	}, nil
}

// LoadImage loads a Docker image archive into the daemon registry from the provided reader.
func (r *DockerRunner) LoadImage(ctx context.Context, reader io.Reader) error {
	resp, err := r.client.ImageLoad(ctx, reader)
	if err != nil {
		return fmt.Errorf("failed to load docker image: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

// ListImages returns a slice of image repository tags present in the local registry.
func (r *DockerRunner) ListImages(ctx context.Context) ([]string, error) {
	images, err := r.client.ImageList(ctx, client.ImageListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list docker images: %w", err)
	}

	var tags []string
	for _, img := range images {
		tags = append(tags, img.RepoTags...)
	}
	return tags, nil
}

// Close closes the underlying Docker engine API client connection.
func (r *DockerRunner) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}
