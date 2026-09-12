// Package handler provides security middleware for authentication, mutual TLS identity extraction, tracing, and webhook HMAC signature validation.
package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// CtxTraceIDKey is the Gin context key for distributed trace IDs.
	CtxTraceIDKey = "TraceID"
	// CtxRequestIDKey is the Gin context key for HTTP request IDs.
	CtxRequestIDKey = "RequestID"
	// CtxCallerIdentityKey is the Gin context key for verified caller identities.
	CtxCallerIdentityKey = "CallerIdentity"
	// CtxUserRoleKey is the Gin context key for caller RBAC roles.
	CtxUserRoleKey = "UserRole"

	// RoleAdmin grants full administrative system access.
	RoleAdmin = "admin"
	// RoleInvoker grants function invocation rights.
	RoleInvoker = "invoker"
)

// TracingMiddleware injects or propagates unique request and trace IDs across the request lifecycle.
func TracingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = uuid.NewString()
		}

		traceID := c.GetHeader("traceparent")
		if traceID == "" {
			traceID = c.GetHeader("X-Trace-ID")
		}
		if traceID == "" {
			traceID = reqID
		}

		c.Set(CtxRequestIDKey, reqID)
		c.Set(CtxTraceIDKey, traceID)

		c.Header("X-Request-ID", reqID)
		c.Header("X-Trace-ID", traceID)

		c.Next()
	}
}

// MTLSIdentityMiddleware extracts the client certificate Subject Common Name from verified mutual TLS handshakes.
func MTLSIdentityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.TLS != nil && len(c.Request.TLS.PeerCertificates) > 0 {
			peer := c.Request.TLS.PeerCertificates[0]
			if peer.Subject.CommonName != "" {
				c.Set(CtxCallerIdentityKey, peer.Subject.CommonName)
			}
		}
		c.Next()
	}
}

// RBACAuthMiddleware enforces role-based access control using API keys or bearer tokens.
func RBACAuthMiddleware(authEnabled bool, adminKey, invokerKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !authEnabled {
			c.Next()
			return
		}

		// Allow public health and metrics scraping unless explicitly restricted
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}

		key := extractAPIKey(c)
		if key == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized: missing API key or bearer token"})
			return
		}

		if adminKey != "" && hmac.Equal([]byte(key), []byte(adminKey)) {
			c.Set(CtxUserRoleKey, RoleAdmin)
			c.Next()
			return
		}

		if invokerKey != "" && hmac.Equal([]byte(key), []byte(invokerKey)) {
			c.Set(CtxUserRoleKey, RoleInvoker)
			// Invokers can only call invoke, hooks, and query invocations
			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/invoke") || strings.HasPrefix(path, "/hooks") || strings.HasPrefix(path, "/invocations") {
				c.Next()
				return
			}

			c.AbortWithStatusJSON(http.StatusForbidden, ErrorResponse{Error: "forbidden: insufficient permissions for this resource"})
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized: invalid API key"})
	}
}

// VerifyWebhookHMAC validates HMAC-SHA256 signatures for webhook payloads against the expected secret.
func VerifyWebhookHMAC(secret string, body []byte, signatureHeader string) bool {
	if secret == "" {
		return true // No secret configured, pass through
	}
	if signatureHeader == "" {
		return false
	}

	sig := strings.TrimPrefix(signatureHeader, "sha256=")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(strings.ToLower(sig)), []byte(strings.ToLower(expectedMAC)))
}

func extractAPIKey(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
		return strings.TrimSpace(apiKey)
	}
	return ""
}
