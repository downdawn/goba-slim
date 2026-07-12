// Package config 提供强类型、可验证的应用配置。
package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	App     AppConfig    `koanf:"app"`
	Server  ServerConfig `koanf:"server"`
	CORS    CORSConfig   `koanf:"cors"`
	Auth    AuthConfig   `koanf:"auth"`
	Log     LogConfig    `koanf:"log"`
	Modules ModuleConfig `koanf:"modules"`
}
type AppConfig struct {
	Environment string `koanf:"environment"`
	Debug       bool   `koanf:"debug"`
	DocsEnabled bool   `koanf:"docs_enabled"`
}
type ServerConfig struct {
	Host            string        `koanf:"host"`
	Port            int           `koanf:"port"`
	HeaderTimeout   time.Duration `koanf:"header_timeout"`
	ReadTimeout     time.Duration `koanf:"read_timeout"`
	WriteTimeout    time.Duration `koanf:"write_timeout"`
	IdleTimeout     time.Duration `koanf:"idle_timeout"`
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout"`
	TrustedProxies  []string      `koanf:"trusted_proxies"`
}
type CORSConfig struct {
	AllowOrigins     []string `koanf:"allow_origins"`
	AllowMethods     []string `koanf:"allow_methods"`
	AllowHeaders     []string `koanf:"allow_headers"`
	AllowCredentials bool     `koanf:"allow_credentials"`
}
type AuthConfig struct {
	Issuer          string        `koanf:"issuer"`
	Audience        string        `koanf:"audience"`
	AccessTokenTTL  time.Duration `koanf:"access_token_ttl"`
	RefreshTokenTTL time.Duration `koanf:"refresh_token_ttl"`
	PrivateKey      Secret        `koanf:"private_key"`
}
type LogConfig struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"`
}
type ModuleConfig struct {
	File         bool `koanf:"file"`
	SystemConfig bool `koanf:"systemconfig"`
}
type Secret string

func NewSecret(value string) Secret { return Secret(value) }
func (s Secret) Reveal() string     { return string(s) }
func (s Secret) String() string {
	if s == "" {
		return ""
	}
	return "[REDACTED]"
}

type Options struct{ File string }

func Default() Config {
	return Config{App: AppConfig{Environment: "development", DocsEnabled: true}, Server: ServerConfig{Host: "0.0.0.0", Port: 8000, HeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, ShutdownTimeout: 15 * time.Second}, CORS: CORSConfig{AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}, AllowHeaders: []string{"Content-Type", "Authorization", "X-Request-ID"}}, Auth: AuthConfig{AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 720 * time.Hour}, Log: LogConfig{Level: "info", Format: "json"}}
}
func Load(_ context.Context, options Options) (Config, error) {
	cfg := Default()
	data := map[string]any{}
	if options.File != "" {
		k := koanf.New(".")
		if err := k.Load(file.Provider(options.File), yaml.Parser()); err != nil {
			return Config{}, fmt.Errorf("读取配置文件: %w", err)
		}
		if err := k.Unmarshal("", &data); err != nil {
			return Config{}, fmt.Errorf("解析配置文件: %w", err)
		}
		applyMap(&cfg, data)
	}
	applyEnvironment(&cfg, os.Environ())
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
func applyMap(cfg *Config, data map[string]any) {
	section := func(name string) map[string]any {
		if value, ok := data[name].(map[string]any); ok {
			return value
		}
		return nil
	}
	setString := func(m map[string]any, key string, target *string) {
		if value, ok := m[key].(string); ok {
			*target = value
		}
	}
	setBool := func(m map[string]any, key string, target *bool) {
		if value, ok := m[key].(bool); ok {
			*target = value
		}
	}
	setInt := func(m map[string]any, key string, target *int) {
		if value, ok := m[key].(int); ok {
			*target = value
		}
		if value, ok := m[key].(float64); ok {
			*target = int(value)
		}
	}
	setDuration := func(m map[string]any, key string, target *time.Duration) {
		if value, ok := m[key].(string); ok {
			if parsed, err := time.ParseDuration(value); err == nil {
				*target = parsed
			}
		}
	}
	if m := section("app"); m != nil {
		setString(m, "environment", &cfg.App.Environment)
		setBool(m, "debug", &cfg.App.Debug)
		setBool(m, "docs_enabled", &cfg.App.DocsEnabled)
	}
	if m := section("server"); m != nil {
		setString(m, "host", &cfg.Server.Host)
		setInt(m, "port", &cfg.Server.Port)
		setDuration(m, "header_timeout", &cfg.Server.HeaderTimeout)
		setDuration(m, "read_timeout", &cfg.Server.ReadTimeout)
		setDuration(m, "write_timeout", &cfg.Server.WriteTimeout)
		setDuration(m, "idle_timeout", &cfg.Server.IdleTimeout)
		setDuration(m, "shutdown_timeout", &cfg.Server.ShutdownTimeout)
	}
	if m := section("auth"); m != nil {
		setString(m, "issuer", &cfg.Auth.Issuer)
		setString(m, "audience", &cfg.Auth.Audience)
		var key string
		setString(m, "private_key", &key)
		if key != "" {
			cfg.Auth.PrivateKey = NewSecret(key)
		}
		setDuration(m, "access_token_ttl", &cfg.Auth.AccessTokenTTL)
		setDuration(m, "refresh_token_ttl", &cfg.Auth.RefreshTokenTTL)
	}
}
func applyEnvironment(cfg *Config, values []string) {
	for _, value := range values {
		key, raw, ok := strings.Cut(value, "=")
		if !ok || !strings.HasPrefix(key, "GOBA_") {
			continue
		}
		switch key {
		case "GOBA_SERVER_PORT":
			if parsed, err := strconv.Atoi(raw); err == nil {
				cfg.Server.Port = parsed
			}
		case "GOBA_SERVER_HOST":
			cfg.Server.Host = raw
		case "GOBA_APP_ENVIRONMENT":
			cfg.App.Environment = raw
		case "GOBA_AUTH_PRIVATE_KEY":
			cfg.Auth.PrivateKey = NewSecret(raw)
		}
	}
}
func (c Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port 必须在 1 到 65535 之间")
	}
	if c.Server.Host == "" {
		return fmt.Errorf("server.host 不能为空")
	}
	if c.CORS.AllowCredentials {
		for _, origin := range c.CORS.AllowOrigins {
			if origin == "*" {
				return fmt.Errorf("cors.allow_origins 使用通配符时不能启用凭据")
			}
		}
	}
	return nil
}
