package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/downdawn/goba-slim/internal/modules/user"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newUserCommand(deps Dependencies) *cobra.Command {
	cmd := &cobra.Command{Use: "user", Short: "管理用户初始化操作"}
	cmd.AddCommand(newCreateAdminCommand(deps))
	return cmd
}

func newCreateAdminCommand(deps Dependencies) *cobra.Command {
	var configFile, username, displayName, email, passwordFile string
	var loadDotEnv, allowMultipleSessions bool
	cmd := &cobra.Command{
		Use:   "create-admin",
		Short: "创建超级管理员",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := deps.Load(cmd.Context(), config.Options{File: configFile, LoadDotEnv: loadDotEnv})
			if err != nil {
				return fmt.Errorf("加载配置失败: %w", err)
			}
			password, err := readAdminPassword(cmd, passwordFile)
			if err != nil {
				return err
			}
			created, err := deps.AddAdmin(cmd.Context(), cfg, user.CreateInput{
				Username: username, Password: password, DisplayName: displayName, Email: email,
				AllowMultipleSessions: allowMultipleSessions,
			})
			if err != nil {
				return fmt.Errorf("创建管理员失败: %w", err)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "管理员 %s 已创建，ID: %s\n", created.Username, created.ID)
			return err
		},
	}
	addConfigFlags(cmd, &configFile, &loadDotEnv)
	cmd.Flags().StringVar(&username, "username", "", "管理员用户名")
	cmd.Flags().StringVar(&displayName, "display-name", "", "显示名（默认使用用户名）")
	cmd.Flags().StringVar(&email, "email", "", "联系邮箱")
	cmd.Flags().StringVar(&passwordFile, "password-file", "", "管理员密码文件路径")
	cmd.Flags().BoolVar(&allowMultipleSessions, "allow-multiple-sessions", false, "允许多个并发会话")
	_ = cmd.MarkFlagRequired("username")
	return cmd
}

func readAdminPassword(cmd *cobra.Command, passwordFile string) (string, error) {
	if passwordFile != "" {
		// #nosec G304 -- 路径由管理员通过显式 CLI 参数提供。
		content, err := os.ReadFile(passwordFile)
		if err != nil {
			return "", fmt.Errorf("读取管理员密码文件失败: %w", err)
		}
		return strings.TrimRight(string(content), "\r\n"), nil
	}
	input, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(int(input.Fd())) {
		return "", fmt.Errorf("非交互环境必须提供 --password-file")
	}
	if _, err := fmt.Fprint(cmd.OutOrStdout(), "管理员密码: "); err != nil {
		return "", err
	}
	first, err := term.ReadPassword(int(input.Fd()))
	if err != nil {
		return "", fmt.Errorf("读取管理员密码失败: %w", err)
	}
	if _, err := fmt.Fprint(cmd.OutOrStdout(), "\n再次输入管理员密码: "); err != nil {
		return "", err
	}
	second, err := term.ReadPassword(int(input.Fd()))
	if err != nil {
		return "", fmt.Errorf("读取管理员密码确认失败: %w", err)
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout())
	if string(first) != string(second) {
		return "", fmt.Errorf("两次输入的管理员密码不一致")
	}
	return string(first), nil
}
