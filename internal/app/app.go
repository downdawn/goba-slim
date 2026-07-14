// Package app 负责应用组合根与生命周期编排。
package app

import (
	"context"

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
	components []lifecycleComponent
	server     server
}

type buildOptions struct {
	components          []lifecycleComponent
	componentsSet       bool
	authService         *auth.Service
	fileService         *filemodule.Service
	systemConfigService *systemconfig.Service
	userService         *user.Service
	server              server
}

// Option 定义构建 App 时可选的显式依赖。
type Option func(*buildOptions)

// withComponents 覆盖默认基础设施生命周期，仅用于 app 包测试。
func withComponents(items ...lifecycleComponent) Option {
	return func(options *buildOptions) {
		options.components = append([]lifecycleComponent(nil), items...)
		options.componentsSet = true
	}
}

// WithServer 覆盖默认 HTTP 服务，仅用于测试注入。
func WithServer(value interface{ Run(context.Context) error }) Option {
	return func(options *buildOptions) {
		options.server = value
	}
}
