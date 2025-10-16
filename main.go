package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main(){
	gin.SetMode(gin.DebugMode)
	r := gin.Default();


	// prometheus
	r.GET("/metrics",gin.WrapH(promhttp.Handler()))
	
	// Lambda Routes

	r.POST("/functions",handleRegister)
	r.GET("/functions",handleList);
	r.POST("/invoke/:id",handleInvoke);
	


	server := &http.Server{
		Addr: ":8300",
		Handler: r,
		ReadTimeout: 20 * time.Second,
		WriteTimeout: 15 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}


	log.Println("Server Running On port 8300");
	log.Fatal(server.ListenAndServe())
}

