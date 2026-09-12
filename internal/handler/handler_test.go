package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"AshitomW/mini-lambda/internal/config"
	"AshitomW/mini-lambda/internal/domain"
	"AshitomW/mini-lambda/internal/handler"
	"AshitomW/mini-lambda/internal/metrics"
	"AshitomW/mini-lambda/internal/repository"
	"AshitomW/mini-lambda/internal/runner"
	"AshitomW/mini-lambda/internal/service"
	"github.com/gin-gonic/gin"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *service.FunctionService) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	tempDir := t.TempDir()
	funcRepo, err := repository.NewFileFunctionRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create func repo: %v", err)
	}

	invRepo := repository.NewMemoryInvocationRepository(0)
	mockRunner := runner.NewMockRunner()
	mockRunner.InvokeFunc = func(_ context.Context, _ domain.Function, payload []byte, _ time.Duration) (*domain.InvocationResult, error) {
		return &domain.InvocationResult{
			Output:     "output:" + string(payload),
			Logs:       "logs",
			DurationMs: 12,
		}, nil
	}

	noopMetrics := metrics.NewNoopMetrics()
	cfg := config.Config{
		MaxConcurrentInvocations: 10,
		DefaultTimeout:           5 * time.Second,
		MaxTimeout:               10 * time.Second,
	}

	funcService := service.NewFunctionService(funcRepo)
	invService := service.NewInvocationService(funcRepo, invRepo, mockRunner, noopMetrics, cfg)
	imgService := service.NewImageService(mockRunner)

	h := handler.NewHandler(funcService, invService, imgService, noopMetrics)
	return handler.NewRouter(h), funcService
}

func TestHealthCheck(t *testing.T) {
	r, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
}

func TestFunctionRegistrationAndRetrieval(t *testing.T) {
	r, _ := setupTestRouter(t)

	payload := []byte(`{"name":"test-fn","image":"python:alpine"}`)
	req := httptest.NewRequest(http.MethodPost, "/functions", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d, body: %s", rec.Code, rec.Body.String())
	}

	var created domain.Function
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created function: %v", err)
	}
	if created.ID == "" || created.Name != "test-fn" {
		t.Fatalf("unexpected function returned: %+v", created)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/functions/"+created.ID, nil)
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", getRec.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/functions", nil)
	listRec := httptest.NewRecorder()
	r.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", listRec.Code)
	}
}

func TestFunctionRegistrationInvalid(t *testing.T) {
	r, _ := setupTestRouter(t)

	payload := []byte(`{"name":""}`)
	req := httptest.NewRequest(http.MethodPost, "/functions", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestInvokeSync(t *testing.T) {
	r, funcService := setupTestRouter(t)
	ctx := context.Background()

	fn, err := funcService.Register(ctx, "echo-func", "alpine")
	if err != nil {
		t.Fatalf("failed to register function: %v", err)
	}

	body := []byte(`{"event":{"msg":"hi"},"timeout":5}`)
	req := httptest.NewRequest(http.MethodPost, "/invoke/"+fn.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body: %s", rec.Code, rec.Body.String())
	}

	var resp handler.SyncInvokeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse sync response: %v", err)
	}
	if resp.Result != `output:{"msg":"hi"}` {
		t.Fatalf("unexpected output: %s", resp.Result)
	}
}

func TestInvokeAsyncAndQuery(t *testing.T) {
	r, funcService := setupTestRouter(t)
	ctx := context.Background()

	fn, err := funcService.Register(ctx, "async-echo", "alpine")
	if err != nil {
		t.Fatalf("failed to register function: %v", err)
	}

	body := []byte(`{"event":{"job":"test"},"timeout":5}`)
	req := httptest.NewRequest(http.MethodPost, "/invoke/"+fn.ID+"/async", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d", rec.Code)
	}

	var asyncResp handler.AsyncInvokeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &asyncResp); err != nil {
		t.Fatalf("failed to parse async response: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	getReq := httptest.NewRequest(http.MethodGet, "/invocations/"+asyncResp.InvocationID, nil)
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /invocations/:id, got %d", getRec.Code)
	}

	postReq := httptest.NewRequest(http.MethodPost, "/invocations/"+asyncResp.InvocationID, nil)
	postRec := httptest.NewRecorder()
	r.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from legacy POST /invocations/:id, got %d", postRec.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/invocations", nil)
	listRec := httptest.NewRecorder()
	r.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /invocations, got %d", listRec.Code)
	}
}

func TestImagesHandlers(t *testing.T) {
	r, _ := setupTestRouter(t)

	listReq := httptest.NewRequest(http.MethodGet, "/images", nil)
	listRec := httptest.NewRecorder()
	r.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /images, got %d", listRec.Code)
	}

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile("image", "test-image.tar")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	_, _ = fw.Write([]byte("fake-tar-data"))
	_ = w.Close()

	uploadReq := httptest.NewRequest(http.MethodPost, "/images", &b)
	uploadReq.Header.Set("Content-Type", w.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	r.ServeHTTP(uploadRec, uploadReq)

	if uploadRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from POST /images, got %d, body: %s", uploadRec.Code, uploadRec.Body.String())
	}
}
