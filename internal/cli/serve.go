package cli

import (
	"fmt"

	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/spf13/cobra"
)

func newServeCommand(deps Dependencies) *cobra.Command {
	var configFile string
	var loadDotEnv bool
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "启动 HTTP 服务",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := deps.Load(cmd.Context(), config.Options{File: configFile, LoadDotEnv: loadDotEnv})
			if err != nil {
				return fmt.Errorf("加载配置失败: %w", err)
			}
			logger, _ := deps.NewLogger(cfg.Log, commandOutput(cmd))
			application, err := deps.Build(cmd.Context(), cfg, logger)
			if err != nil {
				return fmt.Errorf("构建应用失败: %w", err)
			}
			if err := application.Run(cmd.Context()); err != nil {
				return fmt.Errorf("运行服务失败: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&configFile, "config", "", "YAML 配置文件路径")
	cmd.Flags().BoolVar(&loadDotEnv, "load-dotenv", false, "显式加载当前目录 .env（仅本地开发）")
	return cmd
}
