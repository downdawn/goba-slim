// Package cli 提供 GoBA 命令行接口。
package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/downdawn/goba-slim/internal/app"
	"github.com/downdawn/goba-slim/internal/modules/user"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/platform/database"
	"github.com/downdawn/goba-slim/internal/platform/logging"
	"github.com/downdawn/goba-slim/internal/version"
	"github.com/spf13/cobra"
)

type application interface {
	Run(context.Context) error
}

// LoadConfigFunc 定义加载强类型配置的依赖。
type LoadConfigFunc func(context.Context, config.Options) (config.Config, error)

// LoggerFactory 定义创建结构化日志记录器的依赖。
type LoggerFactory func(config.LogConfig, io.Writer) (*slog.Logger, *slog.LevelVar)

// BuildApplicationFunc 定义装配可运行应用的依赖。
type BuildApplicationFunc func(context.Context, config.Config, *slog.Logger) (application, error)
type DatabaseStatusFunc func(context.Context, config.Config) (database.Status, error)
type InitializeDatabaseFunc func(context.Context, config.Config) error
type CreateAdminFunc func(context.Context, config.Config, user.CreateInput) (user.User, error)

// Dependencies 定义命令行所需的可替换依赖，便于测试且避免隐式全局状态。
type Dependencies struct {
	Load      LoadConfigFunc
	NewLogger LoggerFactory
	Build     BuildApplicationFunc
	DBStatus  DatabaseStatusFunc
	DBInit    InitializeDatabaseFunc
	AddAdmin  CreateAdminFunc
}

func (d Dependencies) withDefaults() Dependencies {
	if d.Load == nil {
		d.Load = config.Load
	}
	if d.NewLogger == nil {
		d.NewLogger = logging.New
	}
	if d.Build == nil {
		d.Build = func(ctx context.Context, cfg config.Config, logger *slog.Logger) (application, error) {
			return app.Build(ctx, cfg, logger)
		}
	}
	if d.DBStatus == nil {
		d.DBStatus = app.DatabaseStatus
	}
	if d.DBInit == nil {
		d.DBInit = app.InitializeDatabase
	}
	if d.AddAdmin == nil {
		d.AddAdmin = app.CreateAdmin
	}
	return d
}

// NewRoot 创建可独立测试的根命令。
func NewRoot(deps Dependencies) *cobra.Command {
	deps = deps.withDefaults()
	root := &cobra.Command{
		Use:           "goba",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newVersionCommand())
	root.AddCommand(newConfigCommand(deps))
	root.AddCommand(newServeCommand(deps))
	root.AddCommand(newDatabaseCommand(deps))
	root.AddCommand(newUserCommand(deps))
	return root
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "显示构建版本信息",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), version.Info().String())
			return err
		},
	}
}

func commandOutput(cmd *cobra.Command) io.Writer {
	if output := cmd.OutOrStdout(); output != nil {
		return output
	}
	return os.Stdout
}
