package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"AshitomW/mini-lambda/internal/config"
	"AshitomW/mini-lambda/internal/domain"
	"AshitomW/mini-lambda/internal/metrics"
	"AshitomW/mini-lambda/internal/service"
	"AshitomW/mini-lambda/pkg/k8s"
	"github.com/gin-gonic/gin"
)

// Handler manages HTTP request handling and response serialization.
type Handler struct {
	funcService *service.FunctionService
	invService  *service.InvocationService
	imgService  *service.ImageService
	metrics     metrics.MetricsRecorder
	cfg         config.Config
}

// NewHandler initializes a new Handler instance with the required services.
func NewHandler(
	funcService *service.FunctionService,
	invService *service.InvocationService,
	imgService *service.ImageService,
	metrics metrics.MetricsRecorder,
	cfg config.Config,
) *Handler {
	return &Handler{
		funcService: funcService,
		invService:  invService,
		imgService:  imgService,
		metrics:     metrics,
		cfg:         cfg,
	}
}

// RegisterFunction handles the registration of a new function.
func (h *Handler) RegisterFunction(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body: name and image are required"})
		return
	}

	fn, err := h.funcService.RegisterFunction(c.Request.Context(), domain.Function{
		Name:          req.Name,
		Image:         req.Image,
		Env:           req.Env,
		AllowNetwork:  req.AllowNetwork,
		WebhookSecret: req.WebhookSecret,
		MemoryMB:      req.MemoryMB,
		TimeoutSec:    req.TimeoutSec,
	})
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to register function"})
		return
	}

	c.JSON(http.StatusCreated, fn.Sanitize())
}

// ListFunctions returns all registered functions with secrets redacted.
func (h *Handler) ListFunctions(c *gin.Context) {
	list, err := h.funcService.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to list functions"})
		return
	}

	sanitized := make([]domain.Function, 0, len(list))
	for _, fn := range list {
		sanitized = append(sanitized, fn.Sanitize())
	}
	c.JSON(http.StatusOK, sanitized)
}

// GetFunction retrieves a single function by ID or Name with secrets redacted.
func (h *Handler) GetFunction(c *gin.Context) {
	id := c.Param("id")
	fn, err := h.funcService.GetByNameOrID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrFunctionNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to get function"})
		return
	}
	c.JSON(http.StatusOK, fn.Sanitize())
}

// InvokeSync executes a function synchronously.
func (h *Handler) InvokeSync(c *gin.Context) {
	id := c.Param("id")

	var req InvokeRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid json body"})
		return
	}

	var payload []byte
	var err error
	contentType := "application/json"

	if req.CloudEvent != nil {
		if valErr := req.CloudEvent.Validate(); valErr != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid cloud event envelope: required fields missing"})
			return
		}
		payload, err = json.Marshal(req.CloudEvent)
		if req.CloudEvent.ContentType != "" {
			contentType = req.CloudEvent.ContentType
		}
	} else {
		payload, err = json.Marshal(req.Event)
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "failed to encode event payload"})
		return
	}

	traceID, _ := c.Get(CtxTraceIDKey)
	traceIDStr, _ := traceID.(string)
	if traceIDStr == "" {
		traceIDStr = c.GetHeader("X-Request-ID")
		if traceIDStr == "" {
			traceIDStr = c.GetHeader("traceparent")
		}
	}

	callerID, _ := c.Get(CtxCallerIdentityKey)
	callerIDStr, _ := callerID.(string)

	invCtx := domain.InvocationContext{
		TraceID:        traceIDStr,
		ContentType:    contentType,
		CallerIdentity: callerIDStr,
	}

	timeout := time.Duration(req.Timeout) * time.Second
	res, err := h.invService.InvokeSync(c.Request.Context(), id, payload, invCtx, timeout)
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

	if traceIDStr != "" {
		c.Header("X-Trace-ID", traceIDStr)
	}

	c.JSON(http.StatusOK, SyncInvokeResponse{
		Result:    res.Output,
		Logs:      res.Logs,
		Duration:  res.DurationMs,
		TraceID:   traceIDStr,
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

	var payload []byte
	var err error
	contentType := "application/json"

	if req.CloudEvent != nil {
		if valErr := req.CloudEvent.Validate(); valErr != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid cloud event envelope: required fields missing"})
			return
		}
		payload, err = json.Marshal(req.CloudEvent)
		if req.CloudEvent.ContentType != "" {
			contentType = req.CloudEvent.ContentType
		}
	} else {
		payload, err = json.Marshal(req.Event)
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "failed to encode event payload"})
		return
	}

	traceID, _ := c.Get(CtxTraceIDKey)
	traceIDStr, _ := traceID.(string)
	if traceIDStr == "" {
		traceIDStr = c.GetHeader("X-Request-ID")
		if traceIDStr == "" {
			traceIDStr = c.GetHeader("traceparent")
		}
	}

	callerID, _ := c.Get(CtxCallerIdentityKey)
	callerIDStr, _ := callerID.(string)

	invCtx := domain.InvocationContext{
		TraceID:        traceIDStr,
		ContentType:    contentType,
		CallerIdentity: callerIDStr,
	}

	timeout := time.Duration(req.Timeout) * time.Second
	inv, err := h.invService.InvokeAsync(c.Request.Context(), id, payload, invCtx, timeout)
	if err != nil {
		if errors.Is(err, domain.ErrFunctionNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	if traceIDStr != "" {
		c.Header("X-Trace-ID", traceIDStr)
	}

	c.JSON(http.StatusAccepted, AsyncInvokeResponse{
		InvocationID: inv.ID,
		Status:       string(inv.Status),
		TraceID:      traceIDStr,
		CreatedAt:    inv.CreatedAt,
	})
}

// HandleWebhook ingests direct HTTP webhooks and forwards payloads to the target function.
func (h *Handler) HandleWebhook(c *gin.Context) {
	identifier := c.Param("identifier")

	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "failed to read webhook body: " + err.Error()})
		return
	}

	fn, err := h.funcService.GetByNameOrID(c.Request.Context(), identifier)
	if err != nil {
		if errors.Is(err, domain.ErrFunctionNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	if fn.WebhookSecret != "" {
		sigHeader := c.GetHeader("X-Hub-Signature-256")
		if sigHeader == "" {
			sigHeader = c.GetHeader("X-Signature-SHA256")
		}
		if sigHeader == "" {
			sigHeader = c.GetHeader("X-Signature")
		}
		if !VerifyWebhookHMAC(fn.WebhookSecret, body, sigHeader) {
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized: invalid webhook HMAC signature"})
			return
		}
	}

	traceID, _ := c.Get(CtxTraceIDKey)
	traceIDStr, _ := traceID.(string)
	if traceIDStr == "" {
		traceIDStr = c.GetHeader("X-GitHub-Delivery")
		if traceIDStr == "" {
			traceIDStr = c.GetHeader("X-Request-ID")
		}
	}

	callerID, _ := c.Get(CtxCallerIdentityKey)
	callerIDStr, _ := callerID.(string)

	invCtx := domain.InvocationContext{
		TraceID:        traceIDStr,
		ContentType:    c.ContentType(),
		CallerIdentity: callerIDStr,
	}

	isAsync := c.Query("async") == "true"
	if isAsync {
		inv, err := h.invService.InvokeAsync(c.Request.Context(), identifier, body, invCtx, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}

		c.JSON(http.StatusAccepted, WebhookInvokeResponse{
			Status:       "ACCEPTED",
			Function:     identifier,
			InvocationID: inv.ID,
		})
		return
	}

	res, err := h.invService.InvokeSync(c.Request.Context(), identifier, body, invCtx, 0)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrExecutionTimeout):
			c.JSON(http.StatusGatewayTimeout, ErrorResponse{Error: err.Error()})
		case errors.Is(err, domain.ErrCapacityExceeded):
			c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, WebhookInvokeResponse{
		Status:     "SUCCESS",
		Function:   identifier,
		Result:     res.Output,
		DurationMs: res.DurationMs,
	})
}

// ListDeadLetter returns all async invocations routed to the Dead Letter Queue.
func (h *Handler) ListDeadLetter(c *gin.Context) {
	dlq, err := h.invService.ListDeadLetter(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to list dead letter invocations: " + err.Error()})
		return
	}
	if dlq == nil {
		dlq = []domain.AsyncInvocation{}
	}
	c.JSON(http.StatusOK, dlq)
}

// RetryDeadLetter retries a failed or dead-lettered async invocation.
func (h *Handler) RetryDeadLetter(c *gin.Context) {
	id := c.Param("invocation_id")
	inv, err := h.invService.RetryDeadLetter(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvocationNotFound):
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		case errors.Is(err, domain.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invocation is not eligible for retry"})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to retry dead letter invocation: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusAccepted, AsyncInvokeResponse{
		InvocationID: inv.ID,
		Status:       string(inv.Status),
		TraceID:      inv.TraceID,
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

// GetK8sManifest exports declarative Kubernetes YAML (CRD, Job, or Pod) for a function.
func (h *Handler) GetK8sManifest(c *gin.Context) {
	id := c.Param("id")
	fn, err := h.funcService.GetByNameOrID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrFunctionNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to get function"})
		return
	}

	format := strings.ToLower(c.DefaultQuery("format", "crd"))
	namespace := c.DefaultQuery("namespace", "mini-lambda")

	var manifest string
	var genErr error

	switch format {
	case "job":
		manifest, genErr = k8s.GenerateJobManifest(fn, namespace)
	case "pod":
		manifest, genErr = k8s.GeneratePodManifest(fn, namespace)
	default:
		manifest, genErr = k8s.GenerateCRDManifest(fn, namespace)
	}

	if genErr != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to generate manifest: " + genErr.Error()})
		return
	}

	c.Data(http.StatusOK, "application/x-yaml; charset=utf-8", []byte(manifest))
}
