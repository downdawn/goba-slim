package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/platform/logging"
	"github.com/downdawn/goba-slim/internal/shared/requestmeta"
	"github.com/downdawn/goba-slim/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

const (
	requestIDHeader     = "X-Request-ID"
	defaultMaxBodyBytes = 10 << 20
	minRequestIDLength  = 8
	maxRequestIDLength  = 128

	defaultContentSecurityPolicy = "default-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'"
	swaggerContentSecurityPolicy = "default-src 'self'; style-src 'self' 'unsafe-inline' https://unpkg.com; script-src 'self' 'unsafe-inline' https://unpkg.com; img-src 'self' data:; font-src 'self' https://unpkg.com; connect-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'"
)

func recoveryMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logging.WithContext(ctx.Request.Context(), logger).Error("HTTP 请求发生未处理异常", "method", ctx.Request.Method, "path", ctx.Request.URL.Path)
				httpapi.WriteError(ctx, errRecovered)
			}
		}()
		ctx.Next()
	}
}

var errRecovered = recoveredError{}

type recoveredError struct{}

func (recoveredError) Error() string { return "recovered panic" }

func requestIDMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := ctx.GetHeader(requestIDHeader)
		if !validRequestID(requestID) {
			requestID = newRequestID()
		}
		ctx.Request = ctx.Request.WithContext(requestmeta.WithRequestID(ctx.Request.Context(), requestID))
		ctx.Set(requestIDHeader, requestID)
		ctx.Header(requestIDHeader, requestID)
		ctx.Next()
	}
}

func validRequestID(value string) bool {
	if len(value) < minRequestIDLength || len(value) > maxRequestIDLength {
		return false
	}
	for _, char := range value {
		if !(unicode.IsLetter(char) || unicode.IsDigit(char) || char == '-' || char == '_' || char == '.') {
			return false
		}
	}
	return true
}

func newRequestID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return "request-id-unavailable"
}

func securityHeadersMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Header("X-Content-Type-Options", "nosniff")
		ctx.Header("X-Frame-Options", "DENY")
		ctx.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		if ctx.Request.URL.Path == "/docs" {
			ctx.Header("Content-Security-Policy", swaggerContentSecurityPolicy)
		} else {
			ctx.Header("Content-Security-Policy", defaultContentSecurityPolicy)
		}
		ctx.Next()
	}
}

func corsMiddleware(cors config.CORSConfig) gin.HandlerFunc {
	allowedOrigins := make(map[string]struct{}, len(cors.AllowOrigins))
	for _, origin := range cors.AllowOrigins {
		allowedOrigins[origin] = struct{}{}
	}
	methods := strings.Join(cors.AllowMethods, ", ")
	headers := strings.Join(cors.AllowHeaders, ", ")

	return func(ctx *gin.Context) {
		origin := ctx.GetHeader("Origin")
		if origin == "" {
			ctx.Next()
			return
		}
		if _, ok := allowedOrigins[origin]; !ok {
			ctx.Next()
			return
		}

		ctx.Header("Access-Control-Allow-Origin", origin)
		ctx.Header("Vary", "Origin")
		if cors.AllowCredentials {
			ctx.Header("Access-Control-Allow-Credentials", "true")
		}
		if methods != "" {
			ctx.Header("Access-Control-Allow-Methods", methods)
		}
		if headers != "" {
			ctx.Header("Access-Control-Allow-Headers", headers)
		}
		if ctx.Request.Method == http.MethodOptions && ctx.GetHeader("Access-Control-Request-Method") != "" {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}
		ctx.Next()
	}
}

func bodyLimitMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.Request.Body != nil {
			ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxBytes)
		}
		ctx.Next()
	}
}

func accessLogMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		started := time.Now()
		ctx.Next()
		logging.WithContext(ctx.Request.Context(), logger).Info("HTTP 请求完成",
			"method", ctx.Request.Method,
			"path", ctx.Request.URL.Path,
			"status", ctx.Writer.Status(),
			"client_ip", ctx.ClientIP(),
			"duration_ms", time.Since(started).Milliseconds(),
		)
	}
}

func timeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if timeout <= 0 {
			ctx.Next()
			return
		}
		requestContext, cancel := context.WithTimeout(ctx.Request.Context(), timeout)
		defer cancel()
		ctx.Request = ctx.Request.WithContext(requestContext)
		ctx.Next()
	}
}
