package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/spf13/cobra"
)

func newDatabaseCommand(deps Dependencies) *cobra.Command {
	cmd := &cobra.Command{Use: "db", Short: "检查或初始化 PostgreSQL"}
	cmd.AddCommand(newDatabaseStatusCommand(deps))
	cmd.AddCommand(newDatabaseInitCommand(deps))
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
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "PostgreSQL %s，Schema 版本 %d\n", status.ServerVersion, status.SchemaVersion)
			return err
		},
	}
	addConfigFlags(cmd, &configFile, &loadDotEnv)
	return cmd
}

func newDatabaseInitCommand(deps Dependencies) *cobra.Command {
	var configFile string
	var loadDotEnv, confirmed bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "显式初始化空 PostgreSQL 数据库",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := deps.Load(cmd.Context(), config.Options{File: configFile, LoadDotEnv: loadDotEnv})
			if err != nil {
				return fmt.Errorf("加载配置失败: %w", err)
			}
			if !confirmed {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "将初始化 %s:%d/%s，输入 yes 继续: ", cfg.Database.Host, cfg.Database.Port, cfg.Database.Name); err != nil {
					return err
				}
				scanner := bufio.NewScanner(cmd.InOrStdin())
				if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "yes" {
					return fmt.Errorf("数据库初始化已取消")
				}
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("读取确认输入失败: %w", err)
				}
			}
			if err := deps.DBInit(cmd.Context(), cfg); err != nil {
				return fmt.Errorf("初始化数据库失败: %w", err)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "数据库初始化完成")
			return err
		},
	}
	addConfigFlags(cmd, &configFile, &loadDotEnv)
	cmd.Flags().BoolVar(&confirmed, "yes", false, "确认初始化目标空数据库")
	return cmd
}

func addConfigFlags(cmd *cobra.Command, configFile *string, loadDotEnv *bool) {
	cmd.Flags().StringVar(configFile, "config", "", "YAML 配置文件路径")
	cmd.Flags().BoolVar(loadDotEnv, "load-dotenv", false, "显式加载当前目录 .env（仅本地开发）")
}
