package app

import (
	"context"
	"fmt"

	"github.com/downdawn/goba-slim/internal/modules/user"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/platform/database"
)

func DatabaseStatus(ctx context.Context, cfg config.Config) (database.Status, error) {
	return database.Inspect(ctx, cfg.Database)
}

func InitializeDatabase(ctx context.Context, cfg config.Config) error {
	return database.Initialize(ctx, cfg.Database)
}

func CreateAdmin(ctx context.Context, cfg config.Config, input user.CreateInput) (user.User, error) {
	components, err := newCoreComponents(cfg)
	if err != nil {
		return user.User{}, fmt.Errorf("构造用户服务失败: %w", err)
	}
	if err := components.database.Start(ctx); err != nil {
		return user.User{}, err
	}
	//nolint:contextcheck // 上游命令 Context 可能已取消，连接池关闭必须独立完成。
	defer func() { _ = components.database.Stop(context.Background()) }()
	return components.users.CreateAdmin(ctx, input)
}
