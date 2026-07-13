package httpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/downdawn/goba-slim/api/openapi/generated"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/platform/health"
	"github.com/downdawn/goba-slim/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRouterServesHealthEndpoints(t *testing.T) {
	router := NewRouter(testOptions(health.NewService(nil)))

	live := httptest.NewRecorder()
	router.ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/livez", nil))
	require.Equal(t, http.StatusOK, live.Code)
	require.JSONEq(t, `{"status":"ok"}`, live.Body.String())
	require.NotEmpty(t, live.Header().Get(requestIDHeader))

	ready := httptest.NewRecorder()
	router.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	require.Equal(t, http.StatusOK, ready.Code)
	require.JSONEq(t, `{"ready":true,"checks":{}}`, ready.Body.String())
}

func TestRouterServesSwaggerUIInDevelopment(t *testing.T) {
	router := NewRouter(testOptions(health.NewService(nil)))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/docs", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "text/html")
	require.Contains(t, recorder.Body.String(), "SwaggerUIBundle")
	require.NotContains(t, recorder.Body.String(), "SwaggerUIStandalonePreset")
	require.Contains(t, recorder.Body.String(), "/openapi.json")
	require.Equal(t, "default-src 'self'; style-src 'self' 'unsafe-inline' https://unpkg.com; script-src 'self' 'unsafe-inline' https://unpkg.com; img-src 'self' data:; font-src 'self' https://unpkg.com; connect-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'", recorder.Header().Get("Content-Security-Policy"))

	favicon := httptest.NewRecorder()
	router.ServeHTTP(favicon, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	require.Equal(t, http.StatusNoContent, favicon.Code)
}

func TestOpenAPIOperationsUseDeclaredTags(t *testing.T) {
	specification, err := generated.GetSpec()
	require.NoError(t, err)
	declared := make(map[string]struct{}, len(specification.Tags))
	for _, tag := range specification.Tags {
		declared[tag.Name] = struct{}{}
	}
	require.NotEmpty(t, declared)

	for path, item := range specification.Paths.Map() {
		for method, operation := range item.Operations() {
			require.Len(t, operation.Tags, 1, "%s %s 必须且只能声明一个 OpenAPI tag", method, path)
			_, ok := declared[operation.Tags[0]]
			require.True(t, ok, "%s %s 使用了未声明的 OpenAPI tag", method, path)
		}
	}
}

func TestRouterDoesNotServeDocumentationInProduction(t *testing.T) {
	options := testOptions(health.NewService(nil))
	options.Config.App.Environment = "production"
	router := NewRouter(options)

	for _, path := range []string{"/docs", "/openapi.json"} {
		t.Run(strings.TrimPrefix(path, "/"), func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

			require.Equal(t, http.StatusNotFound, recorder.Code)
		})
	}
}

func TestRouterReportsUnavailableReadiness(t *testing.T) {
	router := NewRouter(testOptions(health.NewService(map[string]health.Check{
		"module:test": func(_ context.Context) error { return errors.New("down") },
	})))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.JSONEq(t, `{"ready":false,"checks":{"module:test":{"status":"down"}}}`, recorder.Body.String())
}

func TestRecoveryReturnsSafeError(t *testing.T) {
	router := NewRouter(testOptions(health.NewService(nil)))
	router.GET("/panic", func(*gin.Context) { panic("secret") })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Contains(t, recorder.Body.String(), "INTERNAL_ERROR")
	require.NotContains(t, recorder.Body.String(), "secret")
	require.NotEmpty(t, recorder.Header().Get(requestIDHeader))
}

func TestRequestIDUsesValidClientValue(t *testing.T) {
	router := NewRouter(testOptions(health.NewService(nil)))
	request := httptest.NewRequest(http.MethodGet, "/livez", nil)
	request.Header.Set(requestIDHeader, "client-request-id-1234")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, "client-request-id-1234", recorder.Header().Get(requestIDHeader))
}

func TestSecurityHeadersArePresent(t *testing.T) {
	router := NewRouter(testOptions(health.NewService(nil)))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/livez", nil))

	require.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "DENY", recorder.Header().Get("X-Frame-Options"))
	require.NotEmpty(t, recorder.Header().Get("Referrer-Policy"))
	require.Equal(t, defaultContentSecurityPolicy, recorder.Header().Get("Content-Security-Policy"))
}

func TestUntrustedProxyCannotSetClientIP(t *testing.T) {
	options := testOptions(health.NewService(nil))
	options.Config.Server.TrustedProxies = []string{"10.0.0.0/8"}
	router := NewRouter(options)
	router.GET("/client-ip", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, ctx.ClientIP())
	})

	request := httptest.NewRequest(http.MethodGet, "/client-ip", nil)
	request.RemoteAddr = "192.0.2.10:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.8")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "192.0.2.10", recorder.Body.String())
}

func TestCORSAcceptsConfiguredOriginWithCredentials(t *testing.T) {
	options := testOptions(health.NewService(nil))
	options.Config.CORS.AllowOrigins = []string{"https://app.example.com"}
	options.Config.CORS.AllowCredentials = true
	router := NewRouter(options)

	request := httptest.NewRequest(http.MethodOptions, "/livez", nil)
	request.Header.Set("Origin", "https://app.example.com")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	request.Header.Set("Access-Control-Request-Headers", "Content-Type")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, "https://app.example.com", recorder.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "true", recorder.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORSRejectsUnconfiguredOrigin(t *testing.T) {
	router := NewRouter(testOptions(health.NewService(nil)))
	request := httptest.NewRequest(http.MethodGet, "/livez", nil)
	request.Header.Set("Origin", "https://attacker.example")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Empty(t, recorder.Header().Get("Access-Control-Allow-Origin"))
}

func testOptions(healthService *health.Service) Options {
	cfg := config.Default()
	cfg.Server.ReadTimeout = time.Second
	return Options{
		Config:  cfg,
		Handler: httpapi.NewHandler(httpapi.HandlerOptions{Health: healthService}),
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}
