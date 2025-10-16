package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)


type RegisterRequest struct{
	Name string `json:"name"`
	Image string `json:"image"`
}

func handleRegister(ctx *gin.Context){
	var req RegisterRequest
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error":"invalid body"})
		return 
	}

	fn := RegisterFunction(req.Name, req.Image)	
	ctx.JSON(200,fn)

}

func handleList(ctx *gin.Context){
	ctx.JSON(http.StatusOK,ListFunction())
}


type InvokeReq struct {
	Event any `json:"event"`
	Timeout int `json:"timeout"`
}


func handleInvoke(ctx *gin.Context){
	id := ctx.Param("id")
	var req InvokeReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest,gin.H{"error":"invalid json"})
		return 
	}

	if req.Timeout == 0 {
		req.Timeout = 120
	}

	fn , err := GetFunction(id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error":err.Error()})
		return 
	}

	res, err := InvokeDockerFunction(fn,req.Event, req.Timeout)
	if err != nil {
		ctx.JSON(500,gin.H{"error":err.Error()})
		return 
	}

	ctx.JSON(http.StatusOK,gin.H{
		"result":res.Output,
		"duration":res.Duration,
		"timestamp":time.Now(),
	})

}