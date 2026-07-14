package app

import (
	"context"
	"fmt"
)

// Run 按固定基础设施顺序启动应用，并始终停止已经启动的组件。
func (a *App) Run(ctx context.Context) error {
	if a == nil || a.server == nil {
		return fmt.Errorf("应用尚未完成装配")
	}
	started, err := startComponents(ctx, a.components)
	if err != nil {
		_ = stopComponents(context.WithoutCancel(ctx), a.components, started)
		return err
	}
	defer func() { _ = stopComponents(context.WithoutCancel(ctx), a.components, started) }()
	return a.server.Run(ctx)
}
