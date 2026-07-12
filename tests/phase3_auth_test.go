//go:build integration

package tests

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"testing"
	"time"

	"github.com/downdawn/goba-slim/api/openapi/generated"
	"github.com/downdawn/goba-slim/internal/app"
	"github.com/downdawn/goba-slim/internal/modules/user"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/platform/database"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestAuthenticationHTTPWorkflow(t *testing.T) {
	cfg := startPostgreSQL(t)
	require.NoError(t, database.Initialize(t.Context(), cfg.Database))
	configureRedis(t, &cfg)
	cfg.Auth.PrivateKey = config.NewSecret(generatePrivateKey(t))
	cfg.CORS.AllowOrigins = []string{"https://app.example.test"}
	cfg.Server.Port = availablePort(t)

	_, err := app.CreateAdmin(t.Context(), cfg, user.CreateInput{Username: "admin", Password: "AdminPassword9"})
	require.NoError(t, err)
	application, err := app.Build(t.Context(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	runResult := make(chan error, 1)
	go func() { runResult <- application.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-runResult)
	})

	baseURL := "http://127.0.0.1:" + strconv.Itoa(cfg.Server.Port)
	waitForReady(t, baseURL)
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}

	login := postJSON(t, client, baseURL+"/api/v1/auth/login", map[string]string{"username": "admin", "password": "AdminPassword9"}, "")
	require.Equal(t, http.StatusOK, login.StatusCode)
	oldCookies := login.Cookies()
	var first generated.TokenResponse
	require.NoError(t, json.NewDecoder(login.Body).Decode(&first))
	require.NoError(t, login.Body.Close())
	require.NotEmpty(t, first.AccessToken)
	require.NotEmpty(t, oldCookies)
	require.True(t, oldCookies[0].HttpOnly)

	me := getAuthorized(t, client, baseURL+"/api/v1/me", first.AccessToken)
	require.Equal(t, http.StatusOK, me.StatusCode)
	require.NoError(t, me.Body.Close())
	users := getAuthorized(t, client, baseURL+"/api/v1/users?page=1&size=20", first.AccessToken)
	require.Equal(t, http.StatusOK, users.StatusCode)
	require.NoError(t, users.Body.Close())

	refresh := postJSON(t, client, baseURL+"/api/v1/auth/refresh", nil, "https://app.example.test")
	require.Equal(t, http.StatusOK, refresh.StatusCode)
	var second generated.TokenResponse
	require.NoError(t, json.NewDecoder(refresh.Body).Decode(&second))
	require.NoError(t, refresh.Body.Close())

	replayRequest, err := http.NewRequestWithContext(t.Context(), http.MethodPost, baseURL+"/api/v1/auth/refresh", nil)
	require.NoError(t, err)
	replayRequest.Header.Set("Origin", "https://app.example.test")
	replayRequest.AddCookie(oldCookies[0])
	replay, err := (&http.Client{Timeout: 10 * time.Second}).Do(replayRequest)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, replay.StatusCode)
	require.NoError(t, replay.Body.Close())

	invalidated := getAuthorized(t, client, baseURL+"/api/v1/me", second.AccessToken)
	require.Equal(t, http.StatusUnauthorized, invalidated.StatusCode)
	require.NoError(t, invalidated.Body.Close())

	loginAgain := postJSON(t, client, baseURL+"/api/v1/auth/login", map[string]string{"username": "admin", "password": "AdminPassword9"}, "")
	require.Equal(t, http.StatusOK, loginAgain.StatusCode)
	var third generated.TokenResponse
	require.NoError(t, json.NewDecoder(loginAgain.Body).Decode(&third))
	require.NoError(t, loginAgain.Body.Close())
	logoutRequest, err := http.NewRequestWithContext(t.Context(), http.MethodPost, baseURL+"/api/v1/auth/logout", nil)
	require.NoError(t, err)
	logoutRequest.Header.Set("Authorization", "Bearer "+third.AccessToken)
	logout, err := client.Do(logoutRequest)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, logout.StatusCode)
	require.NoError(t, logout.Body.Close())
	afterLogout := getAuthorized(t, client, baseURL+"/api/v1/me", third.AccessToken)
	require.Equal(t, http.StatusUnauthorized, afterLogout.StatusCode)
	require.NoError(t, afterLogout.Body.Close())
}

func configureRedis(t *testing.T, cfg *config.Config) {
	t.Helper()
	container, err := tcredis.Run(t.Context(), "redis:7-alpine")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testcontainers.TerminateContainer(container)) })
	host, err := container.Host(t.Context())
	require.NoError(t, err)
	port, err := container.MappedPort(t.Context(), "6379/tcp")
	require.NoError(t, err)
	cfg.Redis.Host = host
	cfg.Redis.Port, err = strconv.Atoi(port.Port())
	require.NoError(t, err)
}

func generatePrivateKey(t *testing.T) string {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}))
}

func availablePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { require.NoError(t, listener.Close()) }()
	return listener.Addr().(*net.TCPAddr).Port
}

func waitForReady(t *testing.T, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(baseURL + "/readyz") //nolint:noctx // 固定本地测试请求受客户端超时和总截止时间约束。
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("服务未在截止时间前就绪")
}

func postJSON(t *testing.T, client *http.Client, target string, body any, origin string) *http.Response {
	t.Helper()
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		require.NoError(t, err)
		payload = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, target, payload)
	require.NoError(t, err)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response, err := client.Do(request)
	require.NoError(t, err)
	return response
}

func getAuthorized(t *testing.T, client *http.Client, target, token string) *http.Response {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	require.NoError(t, err)
	return response
}
