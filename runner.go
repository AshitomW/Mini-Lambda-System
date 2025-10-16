package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type InvocationResult struct {
	Output   string `json:"output"`
	Logs     string `json:"logs"`
	Duration time.Duration `json:duration_ms`
}


func InvokeDockerFunction(fn Function, payload any, timeout int)(*InvocationResult, error){
	ctx, cancel := context.WithTimeout(context.Background(),time.Duration(timeout)*time.Second)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv)

	if err != nil{
		return nil , err
	}

	eventData, _ := json.Marshal(payload)
	start := time.Now()


	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: fn.Image,
		Cmd: []string{},
		Env: []string{},
	},nil,nil,nil,"")

	if err != nil {
		return nil, fmt.Errorf("Create Container : %v",err)
	}

	defer func(){
		_ = cli.ContainerRemove(context.Background(),resp.ID,client.ContainerRemoveOptions{Force: true})
	}()


	// feeding event into container's stdin through attach

	go func(){
		hijack, err := cli.ContainerAttach(ctx,resp.ID,client.ContainerAttachOptions{
			Stdin: true,
			Stdout: true,
			Stderr: true,
			Stream: true,
		})

		if err != nil {
			defer hijack.Close();
			hijack.Conn.Write(eventData)
			hijack.CloseWrite()
		}
	}()



}

