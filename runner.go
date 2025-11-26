package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type InvocationStatus string 


const (
	StatusPending InvocationStatus = "PENDING"
	StatusRunning InvocationStatus = "RUNNING"
	StatusCompleted InvocationStatus = "COMPLETED"
	StatusFailed InvocationStatus = "FAILED"
)


type InvocationResult struct {
	Output   string `json:"output"`
	Logs     string `json:"logs"`
	Duration time.Duration `json:"duration_ms"`
}




type AsyncInvocation struct {
	ID string `json:"id"`
	FunctionID string `json:"function_id"`
	Status InvocationStatus `json:"status"`
	Result *InvocationResult `json:"result,omitempty"`
	Error string `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	mutex sync.Mutex // for per invocation lock
}

var (
	invocationsMutex sync.Mutex
	asyncInvocations = map[string]*AsyncInvocation{}
)


func StartAsyncInvocation(functionID string, payload any, timeout int) *AsyncInvocation{
	invocation := &AsyncInvocation{
		ID: uuid.NewString(),
		FunctionID: functionID,
		Status: StatusPending,
		CreatedAt: time.Now(),
	}

	invocationsMutex.Lock()
	asyncInvocations[invocation.ID] = invocation
	invocationsMutex.Unlock()

	go func(){
		invocation.mutex.Lock()
		invocation.Status = StatusRunning
		invocation.mutex.Unlock()

		fn, err := GetFunction(functionID)

		if err != nil{
			invocation.mutex.Lock()
			invocation.Status = StatusFailed
			invocation.Error = err.Error()
			now := time.Now()
			invocation.CompletedAt = &now 
			invocation.mutex.Unlock()
			return 
		}

		result, err := InvokeDockerFunction(fn,payload,timeout)

		invocation.mutex.Lock()
		now := time.Now()
		invocation.CompletedAt= &now

		if err != nil {
			invocation.Status = StatusFailed
			invocation.Error = err.Error()
		}else{
			invocation.Status = StatusCompleted
			invocation.Result = result 
		}

		invocation.mutex.Unlock()


	}()


	return invocation 


}





// Global Docker client instance
var dockerClient *client.Client

func init() {
	var err error
	dockerClient, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}
}








func LoadDockerImage(tarPath string) error {
	file, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("failed to open tar file: %v", err)
	}
	defer file.Close()

	ctx := context.Background()
	_, err = dockerClient.ImageLoad(ctx, file)
	if err != nil {
		return fmt.Errorf("failed to load image: %v", err)
	}

	return nil
}

// CloseDockerClient closes the Docker client connection
func CloseDockerClient() error {
	if dockerClient != nil {
		return dockerClient.Close()
	}
	return nil
}





func InvokeDockerFunction(fn Function, payload any, timeout int) (*InvocationResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	eventData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %v", err)
	}
	start := time.Now()

	resp, err := dockerClient.ContainerCreate(ctx, &container.Config{
		Image:        fn.Image,
		Cmd:          []string{},
		Env:          []string{},
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    true,
		StdinOnce:    true,
	}, nil, nil, nil, "")

	if err != nil {
		return nil, fmt.Errorf("create container: %v", err)
	}

	defer func() {
		_ = dockerClient.ContainerRemove(context.Background(), resp.ID, client.ContainerRemoveOptions{Force: true})
	}()

	if err := dockerClient.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
		return nil, err
	}

	hijack, err := dockerClient.ContainerAttach(ctx, resp.ID, client.ContainerAttachOptions{
		Stdin:  true,
		Stdout: true,
		Stderr: true,
		Stream: true,
	})

	if err != nil {
		return nil, fmt.Errorf("attach failed: %v", err)
	}
	defer hijack.Close()

	// Write event data
	if len(eventData) > 0 {
		hijack.Conn.Write(eventData)
	}
	hijack.CloseWrite()

	statusCh, errCh := dockerClient.ContainerWait(ctx, resp.ID, container.WaitConditionNextExit)

	select {
	case <-ctx.Done():
		_ = dockerClient.ContainerStop(context.Background(), resp.ID, client.ContainerStopOptions{})
		return nil, fmt.Errorf("timeout exceeded")

	case err := <-errCh:
		if err != nil {
			return nil, err
		}
	case <-statusCh:
	}

	outLogs, _ := dockerClient.ContainerLogs(context.Background(), resp.ID, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	buf := new(bytes.Buffer)
	buf.ReadFrom(outLogs)

	duration := time.Since(start)
	metricsInvocations.WithLabelValues(fn.Name).Inc()
	metricsDuration.WithLabelValues(fn.Name).Observe(duration.Seconds() * 1000)

	return &InvocationResult{
		Output:   buf.String(),
		Logs:     buf.String(),
		Duration: duration / time.Millisecond,
	}, nil
}


func ListDockerImages() ([]string, error) {
	ctx := context.Background()
	images, err := dockerClient.ImageList(ctx, client.ImageListOptions{})
	if err != nil {
		return nil, err
	}

	var imageNames []string

	for _, img := range images {
		imageNames = append(imageNames, img.RepoTags...)
	}

	return imageNames, nil
}