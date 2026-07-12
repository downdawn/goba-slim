package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/spf13/cobra"
)

func newConfigCommand(deps Dependencies) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "检查或显示配置"}
	cmd.AddCommand(newConfigCheckCommand(deps))
	cmd.AddCommand(newConfigPrintCommand(deps))
	return cmd
}

func newConfigCheckCommand(deps Dependencies) *cobra.Command {
	var configFile string
	var loadDotEnv bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "检查配置是否有效",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := deps.Load(cmd.Context(), config.Options{File: configFile, LoadDotEnv: loadDotEnv}); err != nil {
				return fmt.Errorf("配置校验失败: %w", err)
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "配置检查通过")
			return err
		},
	}
	cmd.Flags().StringVar(&configFile, "config", "", "YAML 配置文件路径")
	cmd.Flags().BoolVar(&loadDotEnv, "load-dotenv", false, "显式加载当前目录 .env（仅本地开发）")
	return cmd
}

func newConfigPrintCommand(deps Dependencies) *cobra.Command {
	var configFile string
	var redact bool
	var loadDotEnv bool
	cmd := &cobra.Command{
		Use:   "print",
		Short: "以脱敏形式显示配置",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !redact {
				return fmt.Errorf("必须提供 --redact 才能输出配置")
			}
			cfg, err := deps.Load(cmd.Context(), config.Options{File: configFile, LoadDotEnv: loadDotEnv})
			if err != nil {
				return fmt.Errorf("加载配置失败: %w", err)
			}
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(newRedactedConfig(cfg))
		},
	}
	cmd.Flags().StringVar(&configFile, "config", "", "YAML 配置文件路径")
	cmd.Flags().BoolVar(&redact, "redact", false, "确认以脱敏形式输出")
	cmd.Flags().BoolVar(&loadDotEnv, "load-dotenv", false, "显式加载当前目录 .env（仅本地开发）")
	return cmd
}

type redactedConfig struct {
	App     redactedAppConfig    `json:"app"`
	Server  redactedServerConfig `json:"server"`
	CORS    redactedCORSConfig   `json:"cors"`
	Auth    redactedAuthConfig   `json:"auth"`
	Log     redactedLogConfig    `json:"log"`
	Modules redactedModuleConfig `json:"modules"`
}

type redactedAppConfig struct {
	Environment string `json:"environment"`
	Debug       bool   `json:"debug"`
	DocsEnabled bool   `json:"docs_enabled"`
}
type redactedServerConfig struct {
	Host            string        `json:"host"`
	Port            int           `json:"port"`
	HeaderTimeout   time.Duration `json:"header_timeout"`
	ReadTimeout     time.Duration `json:"read_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout"`
	IdleTimeout     time.Duration `json:"idle_timeout"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	TrustedProxies  []string      `json:"trusted_proxies"`
}
type redactedCORSConfig struct {
	AllowOrigins     []string `json:"allow_origins"`
	AllowMethods     []string `json:"allow_methods"`
	AllowHeaders     []string `json:"allow_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
}
type redactedAuthConfig struct {
	Issuer          string        `json:"issuer"`
	Audience        string        `json:"audience"`
	AccessTokenTTL  time.Duration `json:"access_token_ttl"`
	RefreshTokenTTL time.Duration `json:"refresh_token_ttl"`
	PrivateKey      string        `json:"private_key"`
	PrivateKeyFile  string        `json:"private_key_file"`
}
type redactedLogConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}
type redactedModuleConfig struct {
	File         bool `json:"file"`
	SystemConfig bool `json:"systemconfig"`
}

func newRedactedConfig(cfg config.Config) redactedConfig {
	return redactedConfig{
		App:     redactedAppConfig{Environment: cfg.App.Environment, Debug: cfg.App.Debug, DocsEnabled: cfg.App.DocsEnabled},
		Server:  redactedServerConfig{Host: cfg.Server.Host, Port: cfg.Server.Port, HeaderTimeout: cfg.Server.HeaderTimeout, ReadTimeout: cfg.Server.ReadTimeout, WriteTimeout: cfg.Server.WriteTimeout, IdleTimeout: cfg.Server.IdleTimeout, ShutdownTimeout: cfg.Server.ShutdownTimeout, TrustedProxies: cfg.Server.TrustedProxies},
		CORS:    redactedCORSConfig{AllowOrigins: cfg.CORS.AllowOrigins, AllowMethods: cfg.CORS.AllowMethods, AllowHeaders: cfg.CORS.AllowHeaders, AllowCredentials: cfg.CORS.AllowCredentials},
		Auth:    redactedAuthConfig{Issuer: cfg.Auth.Issuer, Audience: cfg.Auth.Audience, AccessTokenTTL: cfg.Auth.AccessTokenTTL, RefreshTokenTTL: cfg.Auth.RefreshTokenTTL, PrivateKey: cfg.Auth.PrivateKey.String(), PrivateKeyFile: cfg.Auth.PrivateKeyFile},
		Log:     redactedLogConfig{Level: cfg.Log.Level, Format: cfg.Log.Format},
		Modules: redactedModuleConfig{File: cfg.Modules.File, SystemConfig: cfg.Modules.SystemConfig},
	}
}
