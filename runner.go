package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type InvocationResult struct {
	Output   string `json:"output"`
	Logs     string `json:"logs"`
	Duration time.Duration `json:"duration_ms"`
}



func LoadDockerImage(tarPath string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv,client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("Failed to create docker client: %v",err)
	}


	file, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("Failed to open tar file: %v",err)
	}


	defer file.Close()

	ctx:= context.Background()
	_, err = cli.ImageLoad(ctx,file)
	if err != nil {
		return fmt.Errorf("Failed to load image: %v",err)
	}

	return nil
}




func InvokeDockerFunction(fn Function, payload any, timeout int)(*InvocationResult, error){
	ctx, cancel := context.WithTimeout(context.Background(),time.Duration(timeout)*time.Second)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv,client.WithAPIVersionNegotiation())

	if err != nil{
		return nil , err
	}

	eventData, _ := json.Marshal(payload)
	start := time.Now()


	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: fn.Image,
		Cmd: []string{},
		Env: []string{},
		AttachStdin: true,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin: true,
		StdinOnce: true,
	},nil,nil,nil,"")

	if err != nil {
		return nil, fmt.Errorf("create container : %v",err)
	}

	defer func(){
		_ = cli.ContainerRemove(context.Background(),resp.ID,client.ContainerRemoveOptions{Force: true})
	}()

	if err := cli.ContainerStart(ctx,resp.ID, client.ContainerStartOptions{}); err != nil {
		return nil, err;
	}

	hijack, err := cli.ContainerAttach(ctx,resp.ID,client.ContainerAttachOptions{
		Stdin: true,
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

	statusCh, errCh := cli.ContainerWait(ctx,resp.ID,container.WaitConditionNextExit)


	select {
	case <- ctx.Done():
		_ = cli.ContainerStop(context.Background(),resp.ID,client.ContainerStopOptions{})
		return nil, fmt.Errorf("timeout exceeded")
	
	case err := <-errCh:
		if err != nil{
			return nil, err
		}
	case <-statusCh:
	}

	outLogs, _ := cli.ContainerLogs(context.Background(),resp.ID,client.ContainerLogsOptions{ShowStdout: true,ShowStderr: true})
	buf := new(bytes.Buffer)
	buf.ReadFrom(outLogs)

	duration := time.Since(start)
	metricsInvocations.WithLabelValues(fn.Name).Inc()
	metricsDuration.WithLabelValues(fn.Name).Observe(duration.Seconds() * 1000)


	return &InvocationResult{
		Output: buf.String(),
		Logs: buf.String(),
		Duration: duration/time.Millisecond,
	},nil
}


func ListDockerImages()([]string, error){
	cli, err := client.NewClientWithOpts(client.FromEnv,client.WithAPIVersionNegotiation())
	if err != nil{
		return nil, err 
	}

	ctx := context.Background()
	images, err := cli.ImageList(ctx,client.ImageListOptions{})
	if err != nil{
		return nil, err 
	}


	var imageNames[] string 

	for _, img := range images{
		imageNames = append(imageNames,img.RepoTags...)
	}

	return imageNames, nil

}