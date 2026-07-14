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
