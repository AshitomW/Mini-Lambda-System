package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"AshitomW/mini-lambda/internal/domain"
	"AshitomW/mini-lambda/internal/metrics"
	"AshitomW/mini-lambda/internal/service"
	"github.com/gin-gonic/gin"
)

// Handler manages HTTP request handling and response serialization.
type Handler struct {
	funcService *service.FunctionService
	invService  *service.InvocationService
	imgService  *service.ImageService
	metrics     metrics.MetricsRecorder
}

// NewHandler initializes a new Handler instance with the required services.
func NewHandler(
	funcService *service.FunctionService,
	invService *service.InvocationService,
	imgService *service.ImageService,
	metrics metrics.MetricsRecorder,
) *Handler {
	return &Handler{
		funcService: funcService,
		invService:  invService,
		imgService:  imgService,
		metrics:     metrics,
	}
}

// RegisterFunction handles the registration of a new function.
func (h *Handler) RegisterFunction(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body: name and image are required"})
		return
	}

	fn, err := h.funcService.Register(c.Request.Context(), req.Name, req.Image)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to register function"})
		return
	}

	c.JSON(http.StatusCreated, fn)
}

// ListFunctions returns all registered functions.
func (h *Handler) ListFunctions(c *gin.Context) {
	list, err := h.funcService.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to list functions"})
		return
	}
	c.JSON(http.StatusOK, list)
}

// GetFunction retrieves a single function by ID.
func (h *Handler) GetFunction(c *gin.Context) {
	id := c.Param("id")
	fn, err := h.funcService.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrFunctionNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to get function"})
		return
	}
	c.JSON(http.StatusOK, fn)
}

// InvokeSync executes a function synchronously.
func (h *Handler) InvokeSync(c *gin.Context) {
	id := c.Param("id")

	var req InvokeRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid json body"})
		return
	}

	payload, err := json.Marshal(req.Event)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "failed to encode event payload"})
		return
	}

	timeout := time.Duration(req.Timeout) * time.Second
	res, err := h.invService.InvokeSync(c.Request.Context(), id, payload, timeout)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrFunctionNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		case errors.Is(err, domain.ErrExecutionTimeout):
			c.JSON(http.StatusGatewayTimeout, ErrorResponse{Error: err.Error()})
		case errors.Is(err, domain.ErrCapacityExceeded):
			c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, SyncInvokeResponse{
		Result:    res.Output,
		Logs:      res.Logs,
		Duration:  res.DurationMs,
		Timestamp: time.Now().UTC(),
	})
}

// InvokeAsync queues a function for asynchronous execution.
func (h *Handler) InvokeAsync(c *gin.Context) {
	id := c.Param("id")

	var req InvokeRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid json body"})
		return
	}

	payload, err := json.Marshal(req.Event)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "failed to encode event payload"})
		return
	}

	timeout := time.Duration(req.Timeout) * time.Second
	inv, err := h.invService.InvokeAsync(c.Request.Context(), id, payload, timeout)
	if err != nil {
		if errors.Is(err, domain.ErrFunctionNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, AsyncInvokeResponse{
		InvocationID: inv.ID,
		Status:       string(inv.Status),
		CreatedAt:    inv.CreatedAt,
	})
}

// GetAsyncInvocation returns the execution record and results for an async invocation.
func (h *Handler) GetAsyncInvocation(c *gin.Context) {
	id := c.Param("invocation_id")
	inv, err := h.invService.GetAsyncInvocation(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrInvocationNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to get invocation"})
		return
	}

	c.JSON(http.StatusOK, inv)
}

// ListAsyncInvocations returns all async invocation records.
func (h *Handler) ListAsyncInvocations(c *gin.Context) {
	invs, err := h.invService.ListAsyncInvocations(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to list invocations"})
		return
	}
	c.JSON(http.StatusOK, invs)
}

// UploadImage streams a Docker image tar archive to the container daemon.
func (h *Handler) UploadImage(c *gin.Context) {
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "no docker image file provided"})
		return
	}
	defer file.Close()

	if err := h.imgService.UploadImage(c.Request.Context(), file); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to load docker image: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, ImageUploadResponse{
		Message:  "Image Uploaded Successfully",
		FileSize: header.Size,
	})
}

// ListImages returns the available Docker images from the daemon registry.
func (h *Handler) ListImages(c *gin.Context) {
	images, err := h.imgService.ListImages(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to load images"})
		return
	}
	c.JSON(http.StatusOK, images)
}

// HealthCheck responds with current service health status.
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "UP"})
}
