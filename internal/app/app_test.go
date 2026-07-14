package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/platform/httpserver"
	"github.com/stretchr/testify/require"
)

type failingServer struct{ err error }

func (s failingServer) Run(context.Context) error { return s.err }

func TestBuildCleansStartedComponentsWhenStartFails(t *testing.T) {
	startCause := errors.New("component start failed")
	events := []string{}
	first := testComponent("postgresql", &events, nil, nil)
	failing := testComponent("redis", &events, startCause, nil)

	application, err := Build(t.Context(), config.Default(), slog.New(slog.NewTextHandler(io.Discard, nil)), withComponents(first, failing))
	require.NoError(t, err)

	err = application.Run(t.Context())
	require.ErrorIs(t, err, startCause)
	require.Equal(t, []string{"start:postgresql", "start:redis", "stop:postgresql"}, events)
}

func TestAppStopsComponentsWhenServerRunFails(t *testing.T) {
	serverCause := errors.New("server start failed")
	events := []string{}
	component := testComponent("postgresql", &events, nil, nil)

	application, err := Build(t.Context(), config.Default(), slog.New(slog.NewTextHandler(io.Discard, nil)), withComponents(component), WithServer(failingServer{err: serverCause}))
	require.NoError(t, err)

	err = application.Run(t.Context())
	require.ErrorIs(t, err, serverCause)
	require.Equal(t, []string{"start:postgresql", "stop:postgresql"}, events)
}

func TestBuildReturnsServiceUnavailableWhenEnabledComponentHealthFails(t *testing.T) {
	component := testComponent("postgresql", nil, nil, func(context.Context) error { return errors.New("database unavailable") })

	application, err := Build(t.Context(), config.Default(), slog.New(slog.NewTextHandler(io.Discard, nil)), withComponents(component))
	require.NoError(t, err)

	response := httptest.NewRecorder()
	application.server.(*httpserver.Server).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.JSONEq(t, `{"ready":false,"checks":{"postgresql":{"status":"down"}}}`, response.Body.String())
}

func TestBuildReturnsOKWhenEnabledComponentHealthSucceeds(t *testing.T) {
	component := testComponent("redis", nil, nil, func(context.Context) error { return nil })

	application, err := Build(t.Context(), config.Default(), slog.New(slog.NewTextHandler(io.Discard, nil)), withComponents(component))
	require.NoError(t, err)

	response := httptest.NewRecorder()
	application.server.(*httpserver.Server).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"ready":true,"checks":{"redis":{"status":"ok"}}}`, response.Body.String())
}

func testComponent(name string, events *[]string, startErr error, health func(context.Context) error) lifecycleComponent {
	return lifecycleComponent{
		name: name,
		start: func(context.Context) error {
			if events != nil {
				*events = append(*events, "start:"+name)
			}
			return startErr
		},
		stop: func(context.Context) error {
			if events != nil {
				*events = append(*events, "stop:"+name)
			}
			return nil
		},
		health: health,
	}
}
