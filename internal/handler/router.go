package handler

import (
	"github.com/gin-gonic/gin"
)

// NewRouter constructs and configures the Gin engine with all application endpoints.
func NewRouter(h *Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/health", h.HealthCheck)
	r.GET("/metrics", gin.WrapH(h.metrics.Handler()))

	r.POST("/functions", h.RegisterFunction)
	r.GET("/functions", h.ListFunctions)
	r.GET("/functions/:id", h.GetFunction)

	r.POST("/invoke/:id", h.InvokeSync)
	r.POST("/invoke/:id/async", h.InvokeAsync)

	r.GET("/invocations/:invocation_id", h.GetAsyncInvocation)
	r.POST("/invocations/:invocation_id", h.GetAsyncInvocation)
	r.GET("/invocations", h.ListAsyncInvocations)

	r.POST("/images", h.UploadImage)
	r.GET("/images", h.ListImages)

	return r
}
