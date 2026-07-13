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
	App          redactedAppConfig          `json:"app"`
	Server       redactedServerConfig       `json:"server"`
	Database     redactedDatabaseConfig     `json:"database"`
	Redis        redactedRedisConfig        `json:"redis"`
	CORS         redactedCORSConfig         `json:"cors"`
	Auth         redactedAuthConfig         `json:"auth"`
	File         redactedFileConfig         `json:"file"`
	SystemConfig redactedSystemConfigConfig `json:"systemconfig"`
	Log          redactedLogConfig          `json:"log"`
	Modules      redactedModuleConfig       `json:"modules"`
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
type redactedDatabaseConfig struct {
	Host           string        `json:"host"`
	Port           int           `json:"port"`
	Name           string        `json:"name"`
	User           string        `json:"user"`
	Password       string        `json:"password"`
	PasswordFile   string        `json:"password_file"`
	SSLMode        string        `json:"ssl_mode"`
	MinConnections int32         `json:"min_connections"`
	MaxConnections int32         `json:"max_connections"`
	ConnectTimeout time.Duration `json:"connect_timeout"`
	HealthTimeout  time.Duration `json:"health_timeout"`
}
type redactedRedisConfig struct {
	Host           string        `json:"host"`
	Port           int           `json:"port"`
	Database       int           `json:"database"`
	Username       string        `json:"username"`
	Password       string        `json:"password"`
	PasswordFile   string        `json:"password_file"`
	TLS            bool          `json:"tls"`
	PoolSize       int           `json:"pool_size"`
	MinIdleConns   int           `json:"min_idle_connections"`
	ConnectTimeout time.Duration `json:"connect_timeout"`
	ReadTimeout    time.Duration `json:"read_timeout"`
	WriteTimeout   time.Duration `json:"write_timeout"`
	HealthTimeout  time.Duration `json:"health_timeout"`
}
type redactedCORSConfig struct {
	AllowOrigins     []string `json:"allow_origins"`
	AllowMethods     []string `json:"allow_methods"`
	AllowHeaders     []string `json:"allow_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
}
type redactedAuthConfig struct {
	Issuer               string            `json:"issuer"`
	Audience             string            `json:"audience"`
	AccessTokenTTL       time.Duration     `json:"access_token_ttl"`
	RefreshTokenTTL      time.Duration     `json:"refresh_token_ttl"`
	PrivateKey           string            `json:"private_key"`
	PrivateKeyFile       string            `json:"private_key_file"`
	KeyID                string            `json:"key_id"`
	VerificationKeyFiles map[string]string `json:"verification_key_files"`
	RefreshCookie        string            `json:"refresh_cookie"`
	CookieDomain         string            `json:"cookie_domain"`
	CookiePath           string            `json:"cookie_path"`
	CookieSecure         bool              `json:"cookie_secure"`
	CookieSameSite       string            `json:"cookie_same_site"`
	LoginAttempts        int               `json:"login_attempts"`
	LoginWindow          time.Duration     `json:"login_window"`
}
type redactedLogConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}
type redactedFileConfig struct {
	StoragePath   string `json:"storage_path"`
	ImageMaxBytes int64  `json:"image_max_bytes"`
	VideoEnabled  bool   `json:"video_enabled"`
	VideoMaxBytes int64  `json:"video_max_bytes"`
}
type redactedSystemConfigConfig struct {
	CacheTTL time.Duration `json:"cache_ttl"`
}
type redactedModuleConfig struct {
	File         bool `json:"file"`
	SystemConfig bool `json:"systemconfig"`
}

func newRedactedConfig(cfg config.Config) redactedConfig {
	return redactedConfig{
		App:          redactedAppConfig{Environment: cfg.App.Environment, Debug: cfg.App.Debug, DocsEnabled: cfg.App.DocsEnabled},
		Server:       redactedServerConfig{Host: cfg.Server.Host, Port: cfg.Server.Port, HeaderTimeout: cfg.Server.HeaderTimeout, ReadTimeout: cfg.Server.ReadTimeout, WriteTimeout: cfg.Server.WriteTimeout, IdleTimeout: cfg.Server.IdleTimeout, ShutdownTimeout: cfg.Server.ShutdownTimeout, TrustedProxies: cfg.Server.TrustedProxies},
		Database:     redactedDatabaseConfig{Host: cfg.Database.Host, Port: cfg.Database.Port, Name: cfg.Database.Name, User: cfg.Database.User, Password: cfg.Database.Password.String(), PasswordFile: cfg.Database.PasswordFile, SSLMode: cfg.Database.SSLMode, MinConnections: cfg.Database.MinConnections, MaxConnections: cfg.Database.MaxConnections, ConnectTimeout: cfg.Database.ConnectTimeout, HealthTimeout: cfg.Database.HealthTimeout},
		Redis:        redactedRedisConfig{Host: cfg.Redis.Host, Port: cfg.Redis.Port, Database: cfg.Redis.Database, Username: cfg.Redis.Username, Password: cfg.Redis.Password.String(), PasswordFile: cfg.Redis.PasswordFile, TLS: cfg.Redis.TLS, PoolSize: cfg.Redis.PoolSize, MinIdleConns: cfg.Redis.MinIdleConns, ConnectTimeout: cfg.Redis.ConnectTimeout, ReadTimeout: cfg.Redis.ReadTimeout, WriteTimeout: cfg.Redis.WriteTimeout, HealthTimeout: cfg.Redis.HealthTimeout},
		CORS:         redactedCORSConfig{AllowOrigins: cfg.CORS.AllowOrigins, AllowMethods: cfg.CORS.AllowMethods, AllowHeaders: cfg.CORS.AllowHeaders, AllowCredentials: cfg.CORS.AllowCredentials},
		Auth:         redactedAuthConfig{Issuer: cfg.Auth.Issuer, Audience: cfg.Auth.Audience, AccessTokenTTL: cfg.Auth.AccessTokenTTL, RefreshTokenTTL: cfg.Auth.RefreshTokenTTL, PrivateKey: cfg.Auth.PrivateKey.String(), PrivateKeyFile: cfg.Auth.PrivateKeyFile, KeyID: cfg.Auth.KeyID, VerificationKeyFiles: cfg.Auth.VerificationKeyFiles, RefreshCookie: cfg.Auth.RefreshCookie, CookieDomain: cfg.Auth.CookieDomain, CookiePath: cfg.Auth.CookiePath, CookieSecure: cfg.Auth.CookieSecure, CookieSameSite: cfg.Auth.CookieSameSite, LoginAttempts: cfg.Auth.LoginAttempts, LoginWindow: cfg.Auth.LoginWindow},
		File:         redactedFileConfig{StoragePath: cfg.File.StoragePath, ImageMaxBytes: cfg.File.ImageMaxBytes, VideoEnabled: cfg.File.VideoEnabled, VideoMaxBytes: cfg.File.VideoMaxBytes},
		SystemConfig: redactedSystemConfigConfig{CacheTTL: cfg.SystemConfig.CacheTTL},
		Log:          redactedLogConfig{Level: cfg.Log.Level, Format: cfg.Log.Format},
		Modules:      redactedModuleConfig{File: cfg.Modules.File, SystemConfig: cfg.Modules.SystemConfig},
	}
}
