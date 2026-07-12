package logging

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/shared/requestmeta"
	"github.com/stretchr/testify/require"
)

func TestNewCreatesJSONLoggerAtConfiguredLevel(t *testing.T) {
	var buf bytes.Buffer
	logger, level := New(config.LogConfig{Level: "debug", Format: "json"}, &buf)

	logger.Debug("ready", "kind", "json")

	require.Contains(t, buf.String(), `"level":"DEBUG"`)
	require.Contains(t, buf.String(), `"kind":"json"`)
	require.Equal(t, slog.LevelDebug, level.Level())
}

func TestNewCreatesTextLogger(t *testing.T) {
	var buf bytes.Buffer
	logger, _ := New(config.LogConfig{Level: "info", Format: "text"}, &buf)

	logger.Info("ready", "kind", "text")

	require.Contains(t, buf.String(), "level=INFO")
	require.Contains(t, buf.String(), "kind=text")
}

func TestWithContextAddsRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := requestmeta.WithRequestID(context.Background(), "req-1")

	WithContext(ctx, logger).Info("ok")

	require.Contains(t, buf.String(), `"request_id":"req-1"`)
}

func TestWithContextOmitsMissingRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	WithContext(context.Background(), logger).Info("ok")

	require.NotContains(t, buf.String(), "request_id")
}

func TestWithContextOmitsEmptyRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := requestmeta.WithRequestID(context.Background(), "")

	WithContext(ctx, logger).Info("ok")

	require.NotContains(t, buf.String(), "request_id")
}

func TestRedactAttrsRedactsSensitiveValuesRecursively(t *testing.T) {
	var buf bytes.Buffer
	handler := RedactAttrs(slog.NewJSONHandler(&buf, nil))
	logger := slog.New(handler)

	logger.Info("config",
		slog.String("password", "plain-password"),
		slog.Group("auth",
			slog.String("private_key", "plain-private-key"),
			slog.String("Authorization", "Bearer secret-token"),
		),
		slog.Any("nested", map[string]string{"cookie": "plain-cookie"}),
	)

	output := buf.String()
	require.NotContains(t, output, "plain-password")
	require.NotContains(t, output, "plain-private-key")
	require.NotContains(t, output, "Bearer secret-token")
	require.NotContains(t, output, "plain-cookie")
	require.GreaterOrEqual(t, bytes.Count(buf.Bytes(), []byte("[REDACTED]")), 4)
}

func TestRedactAttrsRedactsSensitiveValuesInsideStructs(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(RedactAttrs(slog.NewJSONHandler(&buf, nil)))

	logger.Info("config", slog.Any("config", config.Config{Auth: config.AuthConfig{PrivateKey: config.NewSecret("plain-private-key")}}))

	require.NotContains(t, buf.String(), "plain-private-key")
}

func TestRedactAttrsDoesNotMutateGroupAttrs(t *testing.T) {
	var buf bytes.Buffer
	attrs := []slog.Attr{slog.String("password", "plain-password")}
	group := slog.Attr{Key: "credentials", Value: slog.GroupValue(attrs...)}
	original := group
	logger := slog.New(RedactAttrs(slog.NewJSONHandler(&buf, nil))).With(group).WithGroup("request")

	logger.Info("ok", slog.Group("auth", attrs[0]))

	require.Equal(t, original, group)
	require.Equal(t, "plain-password", attrs[0].Value.String())
	require.Equal(t, "plain-password", group.Value.Group()[0].Value.String())
	require.NotContains(t, buf.String(), "plain-password")
}

func TestRedactAttrsHandlesCyclicMaps(t *testing.T) {
	tests := map[string]func() map[string]any{
		"self reference": func() map[string]any {
			value := map[string]any{"password": "plain-password"}
			value["self"] = value
			return value
		},
		"indirect reference": func() map[string]any {
			first := map[string]any{"api_key": "plain-api-key"}
			second := map[string]any{"parent": first}
			first["child"] = second
			return first
		},
	}

	for name, build := range tests {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(RedactAttrs(slog.NewJSONHandler(&buf, nil)))

			require.NotPanics(t, func() { logger.Info("config", slog.Any("value", build())) })
			require.NotContains(t, buf.String(), "plain-password")
			require.NotContains(t, buf.String(), "plain-api-key")
			require.Contains(t, buf.String(), "[CYCLE]")
		})
	}
}

func TestSensitiveKeyClassification(t *testing.T) {
	sensitive := []string{
		"password", "TOKEN", "Authorization", "cookie", "private.key",
		"access-token", "refresh token", "id_token", "API.KEY", "client-secret",
		"secret", "set cookie",
	}
	metadata := []string{"access_token_ttl", "refresh-token-ttl", "private key file", "token_count", "secret_name"}

	for _, key := range sensitive {
		t.Run("redacts "+key, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(RedactAttrs(slog.NewJSONHandler(&buf, nil)))
			logger.Info("config", key, "credential-value")
			require.NotContains(t, buf.String(), "credential-value")
			require.Contains(t, buf.String(), "[REDACTED]")
		})
	}
	for _, key := range metadata {
		t.Run("keeps "+key, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(RedactAttrs(slog.NewJSONHandler(&buf, nil)))
			logger.Info("config", key, "metadata-value")
			require.Contains(t, buf.String(), "metadata-value")
		})
	}
}

type sensitiveLogValue struct{}

func (sensitiveLogValue) LogValue() slog.Value {
	return slog.GroupValue(slog.String("client_secret", "plain-client-secret"))
}

func TestRedactAttrsResolvesLogValuerGroups(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(RedactAttrs(slog.NewJSONHandler(&buf, nil)))

	logger.Info("config", slog.Any("auth", sensitiveLogValue{}))

	require.NotContains(t, buf.String(), "plain-client-secret")
	require.Contains(t, buf.String(), "[REDACTED]")
}

func TestNewLevelVarUpdatesFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger, level := New(config.LogConfig{Level: "info", Format: "json"}, &buf)

	logger.Debug("before")
	level.Set(slog.LevelDebug)
	logger.Debug("after")

	require.NotContains(t, buf.String(), "before")
	require.Contains(t, buf.String(), "after")
}

func TestRedactAttrsPreservesEnabledFiltering(t *testing.T) {
	var buf bytes.Buffer
	handler := RedactAttrs(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	logger := slog.New(handler)

	require.False(t, handler.Enabled(context.Background(), slog.LevelInfo))
	logger.Info("filtered", "password", "plain-password")
	logger.Warn("included", "password", "plain-password")

	require.NotContains(t, buf.String(), "filtered")
	require.Contains(t, buf.String(), "included")
	require.NotContains(t, buf.String(), "plain-password")
}
