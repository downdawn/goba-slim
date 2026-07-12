package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/downdawn/goba-slim/internal/module"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/platform/httpserver"
	"github.com/stretchr/testify/require"
)

type lifecycleModule struct {
	manifest module.Manifest
	events   *[]string
	startErr error
}

func (m *lifecycleModule) Manifest() module.Manifest { return m.manifest }

func (m *lifecycleModule) Register(*module.Registry) error { return nil }

func (m *lifecycleModule) Start(context.Context) error {
	*m.events = append(*m.events, "start:"+m.manifest.Name)
	return m.startErr
}

func (m *lifecycleModule) Stop(context.Context) error {
	*m.events = append(*m.events, "stop:"+m.manifest.Name)
	return nil
}

type healthModule struct {
	manifest module.Manifest
	health   func(context.Context) error
}

func (m healthModule) Manifest() module.Manifest { return m.manifest }

func (m healthModule) Register(*module.Registry) error { return nil }

func (m healthModule) Health(ctx context.Context) error { return m.health(ctx) }

type failingServer struct{ err error }

func (s failingServer) Run(context.Context) error { return s.err }

func TestBuildCleansStartedModulesWhenModuleStartFails(t *testing.T) {
	startCause := errors.New("module start failed")
	events := []string{}
	first := &lifecycleModule{manifest: module.Manifest{Name: "first", Core: true}, events: &events}
	failing := &lifecycleModule{manifest: module.Manifest{Name: "failing", Core: true, Requires: []string{"first"}}, events: &events, startErr: startCause}

	application, err := Build(t.Context(), config.Default(), slog.New(slog.NewTextHandler(io.Discard, nil)), WithCoreModules(), WithModules(first, failing))
	require.NoError(t, err)

	err = application.Run(t.Context())
	require.ErrorIs(t, err, startCause)
	require.Equal(t, []string{"start:first", "start:failing", "stop:first"}, events)
}

func TestAppStopsRuntimeWhenServerRunFails(t *testing.T) {
	serverCause := errors.New("server start failed")
	events := []string{}
	item := &lifecycleModule{manifest: module.Manifest{Name: "module", Core: true}, events: &events}

	application, err := Build(t.Context(), config.Default(), slog.New(slog.NewTextHandler(io.Discard, nil)), WithCoreModules(), WithModules(item), WithServer(failingServer{err: serverCause}))
	require.NoError(t, err)

	err = application.Run(t.Context())
	require.ErrorIs(t, err, serverCause)
	require.Equal(t, []string{"start:module", "stop:module"}, events)
}

func TestBuildReturnsServiceUnavailableWhenEnabledModuleHealthFails(t *testing.T) {
	item := healthModule{
		manifest: module.Manifest{Name: "database", Core: true},
		health:   func(context.Context) error { return errors.New("database unavailable") },
	}

	application, err := Build(t.Context(), config.Default(), slog.New(slog.NewTextHandler(io.Discard, nil)), WithCoreModules(), WithModules(item))
	require.NoError(t, err)

	response := httptest.NewRecorder()
	application.server.(*httpserver.Server).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.JSONEq(t, `{"ready":false,"checks":{"module:database":{"status":"down"}}}`, response.Body.String())
}

func TestBuildReturnsOKWhenEnabledModuleHealthSucceeds(t *testing.T) {
	item := healthModule{
		manifest: module.Manifest{Name: "cache", Core: true},
		health:   func(context.Context) error { return nil },
	}

	application, err := Build(t.Context(), config.Default(), slog.New(slog.NewTextHandler(io.Discard, nil)), WithCoreModules(), WithModules(item))
	require.NoError(t, err)

	response := httptest.NewRecorder()
	application.server.(*httpserver.Server).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"ready":true,"checks":{"module:cache":{"status":"ok"}}}`, response.Body.String())
}
