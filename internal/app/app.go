// Package app 负责应用组合根与生命周期编排。
package app

import (
	"context"

	"github.com/downdawn/goba-slim/internal/module"
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
	modules []module.Module
	server  server
}

// Option 定义构建 App 时可选的显式依赖。
type Option func(*buildOptions)

// WithModules 为应用装配提供业务模块；当前默认不注册任何业务模块。
func WithModules(items ...module.Module) Option {
	return func(options *buildOptions) {
		options.modules = append(options.modules, items...)
	}
}

// WithServer 覆盖默认 HTTP 服务，仅用于测试注入。
func WithServer(value interface{ Run(context.Context) error }) Option {
	return func(options *buildOptions) {
		options.server = value
	}
}
