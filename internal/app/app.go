// Package app 负责应用组合根与生命周期编排。
package app

import (
	"context"

	"github.com/downdawn/goba-slim/internal/module"
	"github.com/downdawn/goba-slim/internal/modules/auth"
	filemodule "github.com/downdawn/goba-slim/internal/modules/file"
	"github.com/downdawn/goba-slim/internal/modules/systemconfig"
	"github.com/downdawn/goba-slim/internal/modules/user"
)

type server interface {
	Run(context.Context) error
}

// App 保存已完成装配的运行时和 HTTP 服务。
type App struct {
	runtime *module.Runtime
	server  server
}

type buildOptions struct {
	modules             []module.Module
	coreModules         []module.Module
	coreModulesSet      bool
	authService         *auth.Service
	fileService         *filemodule.Service
	systemConfigService *systemconfig.Service
	userService         *user.Service
	server              server
}

// Option 定义构建 App 时可选的显式依赖。
type Option func(*buildOptions)

// WithModules 为应用装配追加业务模块。
func WithModules(items ...module.Module) Option {
	return func(options *buildOptions) {
		options.modules = append(options.modules, items...)
	}
}

// WithCoreModules 覆盖默认核心模块，仅用于测试注入。
func WithCoreModules(items ...module.Module) Option {
	return func(options *buildOptions) {
		options.coreModules = append([]module.Module(nil), items...)
		options.coreModulesSet = true
	}
}

// WithServer 覆盖默认 HTTP 服务，仅用于测试注入。
func WithServer(value interface{ Run(context.Context) error }) Option {
	return func(options *buildOptions) {
		options.server = value
	}
}
