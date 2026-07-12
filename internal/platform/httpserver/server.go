package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/downdawn/goba-slim/internal/platform/config"
)

// Server 将 HTTP 超时策略与基于 Context 的优雅关闭封装在一起。
type Server struct {
	server          *http.Server
	listener        net.Listener
	address         string
	shutdownTimeout time.Duration
	mu              sync.Mutex
	running         bool
}

// ServerOptions 集中传入已构造的监听器、处理器和服务器配置，避免读取全局状态。
type ServerOptions struct {
	Listener net.Listener
	Address  string
	Handler  http.Handler
	Config   config.ServerConfig
}

// NewServer 创建带显式读写和空闲超时的 HTTP Server。Listener 供测试或外部监听器注入。
func NewServer(options ServerOptions) *Server {
	return &Server{
		server: &http.Server{
			Addr:              options.Address,
			Handler:           options.Handler,
			ReadHeaderTimeout: options.Config.HeaderTimeout,
			ReadTimeout:       options.Config.ReadTimeout,
			WriteTimeout:      options.Config.WriteTimeout,
			IdleTimeout:       options.Config.IdleTimeout,
		},
		listener:        options.Listener,
		address:         options.Address,
		shutdownTimeout: options.Config.ShutdownTimeout,
	}
}

// Handler 返回已装配的 HTTP 处理器，供应用级 HTTP 契约测试使用。
func (s *Server) Handler() http.Handler {
	return s.server.Handler
}

// Run 运行 HTTP 服务，并在上游 Context 取消后使用独立超时 Context 等待在途请求结束。
func (s *Server) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("HTTP Server 已在运行")
	}
	s.running = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	listener := s.listener
	if listener == nil {
		var err error
		listener, err = net.Listen("tcp", s.address)
		if err != nil {
			return fmt.Errorf("监听 HTTP 地址 %q 失败: %w", s.address, err)
		}
	}

	errCh := make(chan error, 1)
	go func() { errCh <- s.server.Serve(listener) }()

	select {
	case err := <-errCh:
		return normalizeServeError(err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
		defer cancel()
		shutdownErr := s.server.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			closeErr := s.server.Close()
			serveErr := <-errCh
			return errors.Join(
				fmt.Errorf("关闭 HTTP Server 失败: %w", shutdownErr),
				wrapCloseError(closeErr),
				normalizeServeError(serveErr),
			)
		}
		return normalizeServeError(<-errCh)
	}
}

func normalizeServeError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("运行 HTTP Server 失败: %w", err)
}

func wrapCloseError(err error) error {
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("强制关闭 HTTP Server 失败: %w", err)
}
