package app

import (
	"context"
	"fmt"
)

// Run 按模块生命周期启动应用，并始终停止已经启动的模块。
func (a *App) Run(ctx context.Context) error {
	if a == nil || a.runtime == nil || a.server == nil {
		return fmt.Errorf("应用尚未完成装配")
	}
	if err := a.runtime.Start(ctx); err != nil {
		return err
	}
	defer func() { _ = a.runtime.Stop(context.Background()) }()
	return a.server.Run(ctx)
}
