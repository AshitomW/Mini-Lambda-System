package handler_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"AshitomW/mini-lambda/internal/handler"
	"github.com/gin-gonic/gin"
)

func TestTracingMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(handler.TracingMiddleware())

	var capturedReqID, capturedTraceID string
	r.GET("/test-trace", func(c *gin.Context) {
		capturedReqID = c.GetString(handler.CtxRequestIDKey)
		capturedTraceID = c.GetString(handler.CtxTraceIDKey)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test-trace", nil)
	req.Header.Set("X-Request-ID", "custom-req-id")
	req.Header.Set("traceparent", "custom-trace-id")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if capturedReqID != "custom-req-id" {
		t.Errorf("expected custom-req-id, got %s", capturedReqID)
	}
	if capturedTraceID != "custom-trace-id" {
		t.Errorf("expected custom-trace-id, got %s", capturedTraceID)
	}
	if rec.Header().Get("X-Request-ID") != "custom-req-id" {
		t.Errorf("expected response header X-Request-ID")
	}
}

func TestRBACAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(handler.RBACAuthMiddleware(true, "admin-secret-key", "invoker-secret-key"))

	r.GET("/functions", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.POST("/invoke/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// 1. Missing Key -> 401
	req1 := httptest.NewRequest(http.MethodGet, "/functions", nil)
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for missing key, got %d", rec1.Code)
	}

	// 2. Invalid Key -> 401
	req2 := httptest.NewRequest(http.MethodGet, "/functions", nil)
	req2.Header.Set("X-API-Key", "wrong-key")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for wrong key, got %d", rec2.Code)
	}

	// 3. Admin Key -> 200 on /functions
	req3 := httptest.NewRequest(http.MethodGet, "/functions", nil)
	req3.Header.Set("Authorization", "Bearer admin-secret-key")
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for admin key, got %d", rec3.Code)
	}

	// 4. Invoker Key on /functions -> 403 Forbidden
	req4 := httptest.NewRequest(http.MethodGet, "/functions", nil)
	req4.Header.Set("X-API-Key", "invoker-secret-key")
	rec4 := httptest.NewRecorder()
	r.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for invoker on admin route, got %d", rec4.Code)
	}

	// 5. Invoker Key on /invoke/test -> 200 OK
	req5 := httptest.NewRequest(http.MethodPost, "/invoke/test", nil)
	req5.Header.Set("X-API-Key", "invoker-secret-key")
	rec5 := httptest.NewRecorder()
	r.ServeHTTP(rec5, req5)
	if rec5.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for invoker on /invoke route, got %d", rec5.Code)
	}
}

func TestVerifyWebhookHMAC(t *testing.T) {
	secret := "webhook-token-xyz"
	payload := []byte(`{"action":"release","version":"v1.0"}`)

	// Compute valid HMAC
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	validSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !handler.VerifyWebhookHMAC(secret, payload, validSig) {
		t.Errorf("expected valid signature to verify successfully")
	}

	// Forged signature
	if handler.VerifyWebhookHMAC(secret, payload, "sha256=invalidhash123") {
		t.Errorf("expected forged signature to fail verification")
	}

	// Empty signature when secret is configured
	if handler.VerifyWebhookHMAC(secret, payload, "") {
		t.Errorf("expected empty signature to fail verification")
	}

	// No secret configured -> pass-through
	if !handler.VerifyWebhookHMAC("", payload, "") {
		t.Errorf("expected pass-through when secret is empty")
	}
}
