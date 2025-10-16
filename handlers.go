package main

import (
	"github.com/gin-gonic/gin"
)


type RegisterRequest struct{
	Name string `json:"name"`
	Image string `json:"image"`
}

func handleRegister(ctx *gin.Context){
	
}

func handleList(ctx *gin.Context){}


func handleInvoke(ctx *gin.Context){}