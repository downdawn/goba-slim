package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/downdawn/goba-slim/internal/module"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/platform/health"
	"github.com/downdawn/goba-slim/internal/platform/httpserver"
	"github.com/downdawn/goba-slim/internal/transport/httpapi"
)

// Build 以显式配置和依赖构造应用，不读取全局配置或执行网络副作用。
func Build(_ context.Context, cfg config.Config, logger *slog.Logger, options ...Option) (*App, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger 不能为空")
	}

	buildOptions := buildOptions{}
	for _, option := range options {
		if option != nil {
			option(&buildOptions)
		}
	}

	registry := module.NewRegistry()
	for _, item := range buildOptions.modules {
		if err := registry.Add(item); err != nil {
			return nil, fmt.Errorf("注册模块失败: %w", err)
		}
		if err := item.Register(registry); err != nil {
			return nil, fmt.Errorf("装配模块 %q 失败: %w", item.Manifest().Name, err)
		}
	}
	ordered, err := registry.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("解析模块依赖失败: %w", err)
	}

	if buildOptions.server == nil {
		healthChecks := make(map[string]health.Check)
		for _, item := range ordered {
			checker, ok := item.(module.HealthChecker)
			if !ok {
				continue
			}
			healthChecks["module:"+item.Manifest().Name] = checker.Health
		}
		healthService := health.NewService(healthChecks)
		handler := httpapi.NewHandler(healthService)
		//nolint:contextcheck // 路由构造不启动 I/O，处理请求时由 Server 注入请求 Context。
		router := httpserver.NewRouter(httpserver.Options{Config: cfg, Handler: handler, Logger: logger})
		buildOptions.server = httpserver.NewServer(httpserver.ServerOptions{
			Address: net.JoinHostPort(cfg.Server.Host, fmt.Sprint(cfg.Server.Port)),
			Handler: router,
			Config:  cfg.Server,
		})
	}

	return &App{runtime: module.NewRuntime(ordered), server: buildOptions.server}, nil
}
