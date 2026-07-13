package cli

import (
	"fmt"

	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/spf13/cobra"
)

func newDoctorCommand(deps Dependencies) *cobra.Command {
	var configFile string
	var loadDotEnv bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "诊断配置、依赖与启用模块",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := deps.Load(cmd.Context(), config.Options{File: configFile, LoadDotEnv: loadDotEnv})
			if err != nil {
				return fmt.Errorf("配置检查失败: %w", err)
			}
			report := deps.Doctor(cmd.Context(), cfg)
			for _, check := range report.Checks {
				status := "FAIL"
				if check.OK {
					status = "OK"
				}
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%-4s %-16s %s\n", status, check.Name, check.Message); err != nil {
					return err
				}
			}
			if !report.OK() {
				return fmt.Errorf("doctor 检查未通过")
			}
			return nil
		},
	}
	addConfigFlags(cmd, &configFile, &loadDotEnv)
	return cmd
}

func newModuleCommand(deps Dependencies) *cobra.Command {
	cmd := &cobra.Command{Use: "module", Short: "查看编译内置模块"}
	cmd.AddCommand(newModuleListCommand(deps))
	return cmd
}

func newModuleListCommand(deps Dependencies) *cobra.Command {
	var configFile string
	var loadDotEnv bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出核心与可选模块状态",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := deps.Load(cmd.Context(), config.Options{File: configFile, LoadDotEnv: loadDotEnv})
			if err != nil {
				return fmt.Errorf("加载配置失败: %w", err)
			}
			items := []struct {
				name, kind, dependencies string
				enabled                  bool
			}{
				{name: "database", kind: "core", enabled: true},
				{name: "user", kind: "core", dependencies: "database", enabled: true},
				{name: "redis", kind: "core", enabled: true},
				{name: "auth", kind: "core", dependencies: "redis,user", enabled: true},
				{name: "file", kind: "optional", dependencies: "auth", enabled: cfg.Modules.File},
				{name: "systemconfig", kind: "optional", dependencies: "database,redis,auth", enabled: cfg.Modules.SystemConfig},
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "NAME          TYPE      ENABLED  DEPENDENCIES"); err != nil {
				return err
			}
			for _, item := range items {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%-13s %-9s %-8t %s\n", item.name, item.kind, item.enabled, item.dependencies); err != nil {
					return err
				}
			}
			return nil
		},
	}
	addConfigFlags(cmd, &configFile, &loadDotEnv)
	return cmd
}
