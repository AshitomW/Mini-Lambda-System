package main

import (
	"io"
	"net/http"
	"os"
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

func handleListDockerImages(ctx *gin.Context){
	images, err := ListAvailableDockerImages();
	if err != nil {
		ctx.JSON(http.StatusInternalServerError,gin.H{"error":"Failed to load images"})
		return 
	}
	ctx.JSON(http.StatusOK,images)
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


func handleImageUpload(ctx *gin.Context){
	file, header, err := ctx.Request.FormFile("image")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error":"no docker image file provided"})
		return 
	}
	defer file.Close()
	

	tempFile, err := os.CreateTemp("","docker-image-*.tar")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"Error":"Failed To Create A Temp File, Internal Error!!"})
		return 
	}

	defer os.Remove(tempFile.Name())
	defer tempFile.Close()


	_, err = io.Copy(tempFile,file)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError,gin.H{"Error":"Failed to Save File"})
		return 
	}



	err = LoadDockerImage(tempFile.Name())
	if err != nil{
		ctx.JSON(http.StatusInternalServerError,gin.H{"Error":"Failed to save File"})
		return 
	}


	ctx.JSON(http.StatusOK,gin.H{
		"message":"Image Uploaded Successfully",
		"file_size":header.Size,
	})	

}