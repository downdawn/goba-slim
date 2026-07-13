package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/downdawn/goba-slim/internal/module"
	filemodule "github.com/downdawn/goba-slim/internal/modules/file"
	"github.com/downdawn/goba-slim/internal/modules/systemconfig"
	systemconfigpostgres "github.com/downdawn/goba-slim/internal/modules/systemconfig/postgres"
	systemconfigredis "github.com/downdawn/goba-slim/internal/modules/systemconfig/redis"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/platform/health"
	"github.com/downdawn/goba-slim/internal/platform/httpserver"
	"github.com/downdawn/goba-slim/internal/shared/clock"
	"github.com/downdawn/goba-slim/internal/shared/id"
	"github.com/downdawn/goba-slim/internal/transport/httpapi"
)

// Build 以显式配置和依赖构造应用，不读取全局配置或执行网络副作用。
func Build(_ context.Context, cfg config.Config, logger *slog.Logger, options ...Option) (*App, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger 不能为空")
	}

	buildOptions := buildOptions{}
	var components *coreComponents
	for _, option := range options {
		if option != nil {
			option(&buildOptions)
		}
	}
	if !buildOptions.coreModulesSet {
		coreModules, builtComponents, err := buildCoreModules(cfg)
		if err != nil {
			return nil, fmt.Errorf("构造核心模块失败: %w", err)
		}
		buildOptions.coreModules = coreModules
		components = builtComponents
		buildOptions.authService = components.auth
		buildOptions.userService = components.users
	}
	if cfg.Modules.File {
		store, err := filemodule.NewLocalStore(cfg.File.StoragePath)
		if err != nil {
			return nil, fmt.Errorf("构造文件存储失败: %w", err)
		}
		service, err := filemodule.NewService(store, id.UUIDv7{}, filemodule.Limits{
			ImageMaxBytes: cfg.File.ImageMaxBytes, VideoEnabled: cfg.File.VideoEnabled, VideoMaxBytes: cfg.File.VideoMaxBytes,
		})
		if err != nil {
			return nil, fmt.Errorf("构造文件服务失败: %w", err)
		}
		buildOptions.fileService = service
		buildOptions.modules = append(buildOptions.modules, filemodule.NewModule(store))
	}
	if cfg.Modules.SystemConfig {
		if components == nil {
			return nil, fmt.Errorf("测试覆盖核心模块时不能启用动态配置模块")
		}
		store, err := systemconfigpostgres.New(components.database)
		if err != nil {
			return nil, fmt.Errorf("构造动态配置仓储失败: %w", err)
		}
		cache, err := systemconfigredis.New(components.redis.Client(), cfg.App.Environment, cfg.SystemConfig.CacheTTL)
		if err != nil {
			return nil, fmt.Errorf("构造动态配置缓存失败: %w", err)
		}
		service, err := systemconfig.NewService(store, store, cache, systemconfig.NewBus(), clock.System{})
		if err != nil {
			return nil, fmt.Errorf("构造动态配置服务失败: %w", err)
		}
		buildOptions.systemConfigService = service
		buildOptions.modules = append(buildOptions.modules, systemconfig.NewModule(service))
	}
	buildOptions.modules = append(buildOptions.coreModules, buildOptions.modules...)

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
		handler := httpapi.NewHandler(httpapi.HandlerOptions{Health: healthService, Auth: buildOptions.authService, Files: buildOptions.fileService, SystemConfigs: buildOptions.systemConfigService, Users: buildOptions.userService, AuthConfig: cfg.Auth, CORS: cfg.CORS})
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
