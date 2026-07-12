package httpserver

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/platform/health"
	"github.com/downdawn/goba-slim/internal/transport/httpapi"
	"github.com/stretchr/testify/require"
)

func TestServerShutsDownAfterContextCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	server := NewServer(ServerOptions{
		Listener: listener,
		Handler: NewRouter(Options{
			Config:  config.Default(),
			Handler: httpapi.NewHandler(health.NewService(nil)),
			Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		}),
		Config: config.ServerConfig{
			HeaderTimeout:   time.Second,
			ReadTimeout:     time.Second,
			WriteTimeout:    time.Second,
			IdleTimeout:     time.Second,
			ShutdownTimeout: time.Second,
		},
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	runDone := make(chan error, 1)
	go func() { runDone <- server.Run(ctx) }()

	response, err := (&http.Client{Timeout: time.Second}).Get("http://" + listener.Addr().String() + "/livez")
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode)

	cancel()
	select {
	case err := <-runDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Server.Run 未在关闭期限内返回")
	}
}
