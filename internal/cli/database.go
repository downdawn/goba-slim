package cli

import (
	"fmt"

	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/spf13/cobra"
)

func newDatabaseCommand(deps Dependencies) *cobra.Command {
	cmd := &cobra.Command{Use: "db", Short: "检查或迁移 PostgreSQL"}
	cmd.AddCommand(newDatabaseStatusCommand(deps))
	cmd.AddCommand(newDatabaseMigrateCommand(deps))
	return cmd
}

func newDatabaseStatusCommand(deps Dependencies) *cobra.Command {
	var configFile string
	var loadDotEnv bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "检查 PostgreSQL 与 Schema 状态",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := deps.Load(cmd.Context(), config.Options{File: configFile, LoadDotEnv: loadDotEnv})
			if err != nil {
				return fmt.Errorf("加载配置失败: %w", err)
			}
			status, err := deps.DBStatus(cmd.Context(), cfg)
			if err != nil {
				return fmt.Errorf("检查数据库失败: %w", err)
			}
			if !status.Initialized {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "PostgreSQL %s，Schema 尚未初始化（期望版本 %d）\n", status.ServerVersion, status.Expected)
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "PostgreSQL %s，Schema 版本 %d，待执行 %d\n", status.ServerVersion, status.SchemaVersion, status.Pending)
			return err
		},
	}
	addConfigFlags(cmd, &configFile, &loadDotEnv)
	return cmd
}

func newDatabaseMigrateCommand(deps Dependencies) *cobra.Command {
	var configFile string
	var loadDotEnv bool
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "显式迁移 PostgreSQL Schema",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := deps.Load(cmd.Context(), config.Options{File: configFile, LoadDotEnv: loadDotEnv})
			if err != nil {
				return fmt.Errorf("加载配置失败: %w", err)
			}
			result, err := deps.DBMigrate(cmd.Context(), cfg)
			if err != nil {
				return fmt.Errorf("迁移数据库失败: %w", err)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "数据库 Schema 已就绪：版本 %d -> %d，应用 %d 项迁移\n", result.Previous, result.Current, result.Applied)
			return err
		},
	}
	addConfigFlags(cmd, &configFile, &loadDotEnv)
	return cmd
}

func addConfigFlags(cmd *cobra.Command, configFile *string, loadDotEnv *bool) {
	cmd.Flags().StringVar(configFile, "config", "", "YAML 配置文件路径")
	cmd.Flags().BoolVar(loadDotEnv, "load-dotenv", false, "显式加载当前目录 .env（仅本地开发）")
}
