// Package config 提供强类型、可验证的应用配置。
package config

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	App          AppConfig          `koanf:"app"`
	Server       ServerConfig       `koanf:"server"`
	Database     DatabaseConfig     `koanf:"database"`
	Redis        RedisConfig        `koanf:"redis"`
	CORS         CORSConfig         `koanf:"cors"`
	Auth         AuthConfig         `koanf:"auth"`
	File         FileConfig         `koanf:"file"`
	SystemConfig SystemConfigConfig `koanf:"systemconfig"`
	Log          LogConfig          `koanf:"log"`
	Modules      ModuleConfig       `koanf:"modules"`
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

type DatabaseConfig struct {
	Host           string        `koanf:"host"`
	Port           int           `koanf:"port"`
	Name           string        `koanf:"name"`
	User           string        `koanf:"user"`
	Password       Secret        `koanf:"password"`
	PasswordFile   string        `koanf:"password_file"`
	SSLMode        string        `koanf:"ssl_mode"`
	MinConnections int32         `koanf:"min_connections"`
	MaxConnections int32         `koanf:"max_connections"`
	ConnectTimeout time.Duration `koanf:"connect_timeout"`
	HealthTimeout  time.Duration `koanf:"health_timeout"`
}

type RedisConfig struct {
	Host           string        `koanf:"host"`
	Port           int           `koanf:"port"`
	Database       int           `koanf:"database"`
	Username       string        `koanf:"username"`
	Password       Secret        `koanf:"password"`
	PasswordFile   string        `koanf:"password_file"`
	TLS            bool          `koanf:"tls"`
	PoolSize       int           `koanf:"pool_size"`
	MinIdleConns   int           `koanf:"min_idle_connections"`
	ConnectTimeout time.Duration `koanf:"connect_timeout"`
	ReadTimeout    time.Duration `koanf:"read_timeout"`
	WriteTimeout   time.Duration `koanf:"write_timeout"`
	HealthTimeout  time.Duration `koanf:"health_timeout"`
}

type CORSConfig struct {
	AllowOrigins     []string `koanf:"allow_origins"`
	AllowMethods     []string `koanf:"allow_methods"`
	AllowHeaders     []string `koanf:"allow_headers"`
	AllowCredentials bool     `koanf:"allow_credentials"`
}

type AuthConfig struct {
	Issuer               string            `koanf:"issuer"`
	Audience             string            `koanf:"audience"`
	AccessTokenTTL       time.Duration     `koanf:"access_token_ttl"`
	RefreshTokenTTL      time.Duration     `koanf:"refresh_token_ttl"`
	PrivateKey           Secret            `koanf:"private_key"`
	PrivateKeyFile       string            `koanf:"private_key_file"`
	KeyID                string            `koanf:"key_id"`
	VerificationKeyFiles map[string]string `koanf:"verification_key_files"`
	VerificationKeys     map[string]string `koanf:"-"`
	RefreshCookie        string            `koanf:"refresh_cookie"`
	CookieDomain         string            `koanf:"cookie_domain"`
	CookiePath           string            `koanf:"cookie_path"`
	CookieSecure         bool              `koanf:"cookie_secure"`
	CookieSameSite       string            `koanf:"cookie_same_site"`
	LoginAttempts        int               `koanf:"login_attempts"`
	LoginWindow          time.Duration     `koanf:"login_window"`
}

type LogConfig struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"`
}

type FileConfig struct {
	StoragePath   string `koanf:"storage_path"`
	ImageMaxBytes int64  `koanf:"image_max_bytes"`
	VideoEnabled  bool   `koanf:"video_enabled"`
	VideoMaxBytes int64  `koanf:"video_max_bytes"`
}

type SystemConfigConfig struct {
	CacheTTL time.Duration `koanf:"cache_ttl"`
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

type Options struct {
	File              string
	EnvironmentPrefix string
	LoadDotEnv        bool
}

func Default() Config {
	return Config{
		App: AppConfig{Environment: "development", DocsEnabled: true},
		Server: ServerConfig{
			Host: "0.0.0.0", Port: 8000, HeaderTimeout: 5 * time.Second,
			ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second,
			IdleTimeout: 60 * time.Second, ShutdownTimeout: 15 * time.Second,
		},
		Database: DatabaseConfig{
			Host: "127.0.0.1", Port: 5432, Name: "goba", User: "goba", SSLMode: "disable",
			MinConnections: 1, MaxConnections: 10, ConnectTimeout: 5 * time.Second, HealthTimeout: 2 * time.Second,
		},
		Redis: RedisConfig{
			Host: "127.0.0.1", Port: 6379, PoolSize: 10, MinIdleConns: 1,
			ConnectTimeout: 5 * time.Second, ReadTimeout: 3 * time.Second,
			WriteTimeout: 3 * time.Second, HealthTimeout: 2 * time.Second,
		},
		CORS: CORSConfig{
			AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders: []string{"Content-Type", "Authorization", "X-Request-ID"},
		},
		Auth: AuthConfig{
			Issuer: "goba-slim", Audience: "goba-slim", KeyID: "default",
			AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 720 * time.Hour,
			RefreshCookie: "goba_refresh", CookiePath: "/api/v1/auth", CookieSameSite: "strict",
			LoginAttempts: 5, LoginWindow: time.Minute,
		},
		File: FileConfig{
			StoragePath: "var/uploads", ImageMaxBytes: 5 << 20,
			VideoEnabled: false, VideoMaxBytes: 100 << 20,
		},
		SystemConfig: SystemConfigConfig{CacheTTL: 5 * time.Minute},
		Log:          LogConfig{Level: "info", Format: "json"},
	}
}

func Load(_ context.Context, options Options) (Config, error) {
	cfg := Default()
	if options.File != "" {
		if err := loadYAML(&cfg, options.File); err != nil {
			return Config{}, err
		}
	}

	prefix := options.EnvironmentPrefix
	if prefix == "" {
		prefix = "GOBA_"
	}
	values := os.Environ()
	if options.LoadDotEnv {
		dotEnv, err := loadDotEnv(".env")
		if err != nil && !os.IsNotExist(err) {
			return Config{}, fmt.Errorf("读取 .env 失败: %w", err)
		}
		values = append(dotEnv, values...)
	}
	if err := applyEnvironment(&cfg, values, prefix); err != nil {
		return Config{}, err
	}
	if options.LoadDotEnv && cfg.App.Environment == "production" {
		return Config{}, fmt.Errorf("app.environment 为 production 时不能加载 .env")
	}
	if err := resolveSecrets(&cfg, prefix); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func loadYAML(cfg *Config, path string) error {
	k := koanf.New(".")
	if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
		return fmt.Errorf("读取配置文件: %w", err)
	}
	decoderConfig := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
		),
		ErrorUnused:      true,
		WeaklyTypedInput: false,
	}
	if err := k.UnmarshalWithConf("", cfg, koanf.UnmarshalConf{DecoderConfig: decoderConfig}); err != nil {
		return fmt.Errorf("解析配置文件: %w", err)
	}
	return nil
}

func loadDotEnv(path string) ([]string, error) {
	// #nosec G304 -- 仅在调用方显式选择加载本地 .env 时读取固定文件名。
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var values []string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf(".env 包含无效配置行")
		}
		values = append(values, key+"="+strings.Trim(strings.TrimSpace(value), "\"'"))
	}
	return values, nil
}

func applyEnvironment(cfg *Config, values []string, prefix string) error {
	env := make(map[string]string)
	for _, value := range values {
		key, raw, ok := strings.Cut(value, "=")
		if ok && strings.HasPrefix(key, prefix) {
			env[key] = raw
		}
	}
	setString := func(key string, target *string) {
		if value, ok := env[prefix+key]; ok {
			*target = value
		}
	}
	setBool := func(key string, target *bool) error {
		if value, ok := env[prefix+key]; ok {
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return invalidEnvironmentValue(prefix + key)
			}
			*target = parsed
		}
		return nil
	}
	setInt := func(key string, target *int) error {
		if value, ok := env[prefix+key]; ok {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return invalidEnvironmentValue(prefix + key)
			}
			*target = parsed
		}
		return nil
	}
	setInt32 := func(key string, target *int32) error {
		if value, ok := env[prefix+key]; ok {
			parsed, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return invalidEnvironmentValue(prefix + key)
			}
			*target = int32(parsed)
		}
		return nil
	}
	setInt64 := func(key string, target *int64) error {
		if value, ok := env[prefix+key]; ok {
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return invalidEnvironmentValue(prefix + key)
			}
			*target = parsed
		}
		return nil
	}
	setDuration := func(key string, target *time.Duration) error {
		if value, ok := env[prefix+key]; ok {
			parsed, err := time.ParseDuration(value)
			if err != nil {
				return invalidEnvironmentValue(prefix + key)
			}
			*target = parsed
		}
		return nil
	}
	setStrings := func(key string, target *[]string) {
		if value, ok := env[prefix+key]; ok {
			*target = splitStrings(value)
		}
	}

	setString("APP_ENVIRONMENT", &cfg.App.Environment)
	if err := setBool("APP_DEBUG", &cfg.App.Debug); err != nil {
		return err
	}
	if err := setBool("APP_DOCS_ENABLED", &cfg.App.DocsEnabled); err != nil {
		return err
	}
	setString("SERVER_HOST", &cfg.Server.Host)
	if err := setInt("SERVER_PORT", &cfg.Server.Port); err != nil {
		return err
	}
	for _, item := range []struct {
		key    string
		target *time.Duration
	}{{"SERVER_HEADER_TIMEOUT", &cfg.Server.HeaderTimeout}, {"SERVER_READ_TIMEOUT", &cfg.Server.ReadTimeout}, {"SERVER_WRITE_TIMEOUT", &cfg.Server.WriteTimeout}, {"SERVER_IDLE_TIMEOUT", &cfg.Server.IdleTimeout}, {"SERVER_SHUTDOWN_TIMEOUT", &cfg.Server.ShutdownTimeout}, {"AUTH_ACCESS_TOKEN_TTL", &cfg.Auth.AccessTokenTTL}, {"AUTH_REFRESH_TOKEN_TTL", &cfg.Auth.RefreshTokenTTL}} {
		if err := setDuration(item.key, item.target); err != nil {
			return err
		}
	}
	setStrings("SERVER_TRUSTED_PROXIES", &cfg.Server.TrustedProxies)
	setString("DATABASE_HOST", &cfg.Database.Host)
	if err := setInt("DATABASE_PORT", &cfg.Database.Port); err != nil {
		return err
	}
	setString("DATABASE_NAME", &cfg.Database.Name)
	setString("DATABASE_USER", &cfg.Database.User)
	setString("DATABASE_PASSWORD", (*string)(&cfg.Database.Password))
	setString("DATABASE_PASSWORD_FILE", &cfg.Database.PasswordFile)
	setString("DATABASE_SSL_MODE", &cfg.Database.SSLMode)
	if err := setInt32("DATABASE_MIN_CONNECTIONS", &cfg.Database.MinConnections); err != nil {
		return err
	}
	if err := setInt32("DATABASE_MAX_CONNECTIONS", &cfg.Database.MaxConnections); err != nil {
		return err
	}
	if err := setDuration("DATABASE_CONNECT_TIMEOUT", &cfg.Database.ConnectTimeout); err != nil {
		return err
	}
	if err := setDuration("DATABASE_HEALTH_TIMEOUT", &cfg.Database.HealthTimeout); err != nil {
		return err
	}
	setString("REDIS_HOST", &cfg.Redis.Host)
	if err := setInt("REDIS_PORT", &cfg.Redis.Port); err != nil {
		return err
	}
	if err := setInt("REDIS_DATABASE", &cfg.Redis.Database); err != nil {
		return err
	}
	setString("REDIS_USERNAME", &cfg.Redis.Username)
	setString("REDIS_PASSWORD", (*string)(&cfg.Redis.Password))
	setString("REDIS_PASSWORD_FILE", &cfg.Redis.PasswordFile)
	if err := setBool("REDIS_TLS", &cfg.Redis.TLS); err != nil {
		return err
	}
	if err := setInt("REDIS_POOL_SIZE", &cfg.Redis.PoolSize); err != nil {
		return err
	}
	if err := setInt("REDIS_MIN_IDLE_CONNECTIONS", &cfg.Redis.MinIdleConns); err != nil {
		return err
	}
	for _, item := range []struct {
		key    string
		target *time.Duration
	}{{"REDIS_CONNECT_TIMEOUT", &cfg.Redis.ConnectTimeout}, {"REDIS_READ_TIMEOUT", &cfg.Redis.ReadTimeout}, {"REDIS_WRITE_TIMEOUT", &cfg.Redis.WriteTimeout}, {"REDIS_HEALTH_TIMEOUT", &cfg.Redis.HealthTimeout}} {
		if err := setDuration(item.key, item.target); err != nil {
			return err
		}
	}
	setStrings("CORS_ALLOW_ORIGINS", &cfg.CORS.AllowOrigins)
	setStrings("CORS_ALLOW_METHODS", &cfg.CORS.AllowMethods)
	setStrings("CORS_ALLOW_HEADERS", &cfg.CORS.AllowHeaders)
	if err := setBool("CORS_ALLOW_CREDENTIALS", &cfg.CORS.AllowCredentials); err != nil {
		return err
	}
	setString("AUTH_ISSUER", &cfg.Auth.Issuer)
	setString("AUTH_AUDIENCE", &cfg.Auth.Audience)
	setString("AUTH_PRIVATE_KEY", (*string)(&cfg.Auth.PrivateKey))
	setString("AUTH_PRIVATE_KEY_FILE", &cfg.Auth.PrivateKeyFile)
	setString("AUTH_KEY_ID", &cfg.Auth.KeyID)
	if value, ok := env[prefix+"AUTH_VERIFICATION_KEY_FILES"]; ok {
		parsed, err := parseKeyFileEntries(value)
		if err != nil {
			return invalidEnvironmentValue(prefix + "AUTH_VERIFICATION_KEY_FILES")
		}
		cfg.Auth.VerificationKeyFiles = parsed
	}
	setString("AUTH_REFRESH_COOKIE", &cfg.Auth.RefreshCookie)
	setString("AUTH_COOKIE_DOMAIN", &cfg.Auth.CookieDomain)
	setString("AUTH_COOKIE_PATH", &cfg.Auth.CookiePath)
	if err := setBool("AUTH_COOKIE_SECURE", &cfg.Auth.CookieSecure); err != nil {
		return err
	}
	setString("AUTH_COOKIE_SAME_SITE", &cfg.Auth.CookieSameSite)
	if err := setInt("AUTH_LOGIN_ATTEMPTS", &cfg.Auth.LoginAttempts); err != nil {
		return err
	}
	if err := setDuration("AUTH_LOGIN_WINDOW", &cfg.Auth.LoginWindow); err != nil {
		return err
	}
	setString("LOG_LEVEL", &cfg.Log.Level)
	setString("LOG_FORMAT", &cfg.Log.Format)
	setString("FILE_STORAGE_PATH", &cfg.File.StoragePath)
	if err := setInt64("FILE_IMAGE_MAX_BYTES", &cfg.File.ImageMaxBytes); err != nil {
		return err
	}
	if err := setBool("FILE_VIDEO_ENABLED", &cfg.File.VideoEnabled); err != nil {
		return err
	}
	if err := setInt64("FILE_VIDEO_MAX_BYTES", &cfg.File.VideoMaxBytes); err != nil {
		return err
	}
	if err := setDuration("SYSTEMCONFIG_CACHE_TTL", &cfg.SystemConfig.CacheTTL); err != nil {
		return err
	}
	if err := setBool("MODULES_FILE", &cfg.Modules.File); err != nil {
		return err
	}
	return setBool("MODULES_SYSTEMCONFIG", &cfg.Modules.SystemConfig)
}

func invalidEnvironmentValue(key string) error {
	return fmt.Errorf("环境变量 %s 格式无效", key)
}

func splitStrings(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseKeyFileEntries(value string) (map[string]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	result := make(map[string]string)
	for _, entry := range strings.Split(value, ",") {
		keyID, path, ok := strings.Cut(strings.TrimSpace(entry), "=")
		keyID, path = strings.TrimSpace(keyID), strings.TrimSpace(path)
		if !ok || keyID == "" || path == "" {
			return nil, fmt.Errorf("公钥文件映射格式无效")
		}
		if _, exists := result[keyID]; exists {
			return nil, fmt.Errorf("公钥 key_id 重复")
		}
		result[keyID] = path
	}
	return result, nil
}

func resolveSecrets(cfg *Config, prefix string) error {
	if err := resolveSecret(&cfg.Database.Password, &cfg.Database.PasswordFile, prefix+"DATABASE_PASSWORD"); err != nil {
		return err
	}
	if err := resolveSecret(&cfg.Redis.Password, &cfg.Redis.PasswordFile, prefix+"REDIS_PASSWORD"); err != nil {
		return err
	}

	if cfg.Auth.PrivateKey != "" && cfg.Auth.PrivateKeyFile != "" {
		return fmt.Errorf("%sAUTH_PRIVATE_KEY 与 %sAUTH_PRIVATE_KEY_FILE 只能配置一种来源", prefix, prefix)
	}
	if cfg.Auth.PrivateKeyFile != "" {
		// #nosec G304 -- 文件路径由部署方通过显式 Secret 配置提供。
		content, err := os.ReadFile(cfg.Auth.PrivateKeyFile)
		if err != nil {
			return fmt.Errorf("读取 %sAUTH_PRIVATE_KEY_FILE 失败: %w", prefix, err)
		}
		cfg.Auth.PrivateKey = NewSecret(strings.TrimRight(string(content), "\r\n"))
	}
	if len(cfg.Auth.VerificationKeyFiles) == 0 {
		return nil
	}
	cfg.Auth.VerificationKeys = make(map[string]string, len(cfg.Auth.VerificationKeyFiles))
	for keyID, path := range cfg.Auth.VerificationKeyFiles {
		// #nosec G304 -- 公钥路径由部署方通过显式轮换配置提供。
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("读取 AUTH_VERIFICATION_KEY_FILES[%s] 失败: %w", keyID, err)
		}
		cfg.Auth.VerificationKeys[keyID] = strings.TrimRight(string(content), "\r\n")
	}
	return nil
}

func resolveSecret(secret *Secret, filePath *string, key string) error {
	if *secret != "" && *filePath != "" {
		return fmt.Errorf("%s 与 %s_FILE 只能配置一种来源", key, key)
	}
	if *filePath == "" {
		return nil
	}
	// #nosec G304 -- 文件路径由部署方通过显式 Secret 配置提供。
	content, err := os.ReadFile(*filePath)
	if err != nil {
		return fmt.Errorf("读取 %s_FILE 失败: %w", key, err)
	}
	*secret = NewSecret(strings.TrimRight(string(content), "\r\n"))
	return nil
}

func (c Config) Validate() error {
	if c.App.Environment != "development" && c.App.Environment != "test" && c.App.Environment != "production" {
		return fmt.Errorf("app.environment 必须是 development、test 或 production")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port 必须在 1 到 65535 之间")
	}
	if c.Server.Host == "" {
		return fmt.Errorf("server.host 不能为空")
	}
	if c.Database.Host == "" {
		return fmt.Errorf("database.host 不能为空")
	}
	if c.Database.Port < 1 || c.Database.Port > 65535 {
		return fmt.Errorf("database.port 必须在 1 到 65535 之间")
	}
	if c.Database.Name == "" || c.Database.User == "" {
		return fmt.Errorf("database.name 和 database.user 不能为空")
	}
	allowedSSLMode := map[string]bool{
		"disable": true, "allow": true, "prefer": true, "require": true, "verify-ca": true, "verify-full": true,
	}
	if !allowedSSLMode[c.Database.SSLMode] {
		return fmt.Errorf("database.ssl_mode 无效")
	}
	if c.Database.MinConnections < 0 || c.Database.MaxConnections < 1 || c.Database.MinConnections > c.Database.MaxConnections {
		return fmt.Errorf("database 连接池大小无效")
	}
	if c.Database.ConnectTimeout <= 0 || c.Database.HealthTimeout <= 0 {
		return fmt.Errorf("database 超时时间必须大于 0")
	}
	if c.Redis.Host == "" || c.Redis.Port < 1 || c.Redis.Port > 65535 || c.Redis.Database < 0 {
		return fmt.Errorf("redis 连接配置无效")
	}
	if c.Redis.PoolSize < 1 || c.Redis.MinIdleConns < 0 || c.Redis.MinIdleConns > c.Redis.PoolSize {
		return fmt.Errorf("redis 连接池大小无效")
	}
	if c.Redis.ConnectTimeout <= 0 || c.Redis.ReadTimeout <= 0 || c.Redis.WriteTimeout <= 0 || c.Redis.HealthTimeout <= 0 {
		return fmt.Errorf("redis 超时时间必须大于 0")
	}
	for _, item := range []struct {
		field string
		value time.Duration
	}{{"server.header_timeout", c.Server.HeaderTimeout}, {"server.read_timeout", c.Server.ReadTimeout}, {"server.write_timeout", c.Server.WriteTimeout}, {"server.idle_timeout", c.Server.IdleTimeout}, {"server.shutdown_timeout", c.Server.ShutdownTimeout}} {
		if item.value <= 0 {
			return fmt.Errorf("%s 必须大于 0", item.field)
		}
	}
	if c.Auth.AccessTokenTTL <= 0 || c.Auth.AccessTokenTTL >= c.Auth.RefreshTokenTTL {
		return fmt.Errorf("auth.access_token_ttl 必须大于 0 且小于 auth.refresh_token_ttl")
	}
	if c.Auth.RefreshTokenTTL <= 0 {
		return fmt.Errorf("auth.refresh_token_ttl 必须大于 0")
	}
	if c.Auth.Issuer == "" || c.Auth.Audience == "" || c.Auth.KeyID == "" {
		return fmt.Errorf("auth issuer、audience 和 key_id 不能为空")
	}
	if _, exists := c.Auth.VerificationKeyFiles[c.Auth.KeyID]; exists {
		return fmt.Errorf("auth.verification_key_files 不能包含当前 key_id")
	}
	for keyID, path := range c.Auth.VerificationKeyFiles {
		if strings.TrimSpace(keyID) == "" || strings.TrimSpace(path) == "" {
			return fmt.Errorf("auth.verification_key_files 包含无效映射")
		}
	}
	if c.Auth.RefreshCookie == "" || c.Auth.CookiePath == "" || (c.Auth.CookieSameSite != "strict" && c.Auth.CookieSameSite != "lax" && c.Auth.CookieSameSite != "none") {
		return fmt.Errorf("auth Cookie 配置无效")
	}
	if c.Auth.CookieSameSite == "none" && !c.Auth.CookieSecure {
		return fmt.Errorf("auth.cookie_same_site 为 none 时必须启用 secure")
	}
	if c.Auth.LoginAttempts < 1 || c.Auth.LoginWindow <= 0 {
		return fmt.Errorf("auth 登录限流配置无效")
	}
	if c.CORS.AllowCredentials {
		for _, origin := range c.CORS.AllowOrigins {
			if origin == "*" {
				return fmt.Errorf("cors.allow_origins 使用通配符时不能启用凭据")
			}
		}
	}
	for _, proxy := range c.Server.TrustedProxies {
		if _, _, err := net.ParseCIDR(proxy); err != nil {
			return fmt.Errorf("server.trusted_proxies 包含无效 CIDR")
		}
	}
	if c.App.Environment == "production" && c.App.Debug {
		return fmt.Errorf("app.debug 在 production 环境必须为 false")
	}
	if c.App.Environment == "production" && c.App.DocsEnabled {
		return fmt.Errorf("app.docs_enabled 在 production 环境必须为 false")
	}
	if c.App.Environment == "production" && (c.Database.SSLMode == "disable" || c.Database.SSLMode == "allow" || c.Database.SSLMode == "prefer") {
		return fmt.Errorf("database.ssl_mode 在 production 环境必须启用 TLS")
	}
	if c.App.Environment == "production" && (!c.Auth.CookieSecure || !c.Redis.TLS) {
		return fmt.Errorf("auth Cookie 与 Redis 在 production 环境必须启用安全传输")
	}
	if c.Log.Format != "json" && c.Log.Format != "text" {
		return fmt.Errorf("log.format 必须是 json 或 text")
	}
	if strings.TrimSpace(c.File.StoragePath) == "" {
		return fmt.Errorf("file.storage_path 不能为空")
	}
	if c.File.ImageMaxBytes < 512 || c.File.ImageMaxBytes > 100<<20 {
		return fmt.Errorf("file.image_max_bytes 必须在 512 到 104857600 之间")
	}
	if c.File.VideoMaxBytes < 512 || c.File.VideoMaxBytes > 2<<30 {
		return fmt.Errorf("file.video_max_bytes 必须在 512 到 2147483648 之间")
	}
	if c.SystemConfig.CacheTTL <= 0 || c.SystemConfig.CacheTTL > 24*time.Hour {
		return fmt.Errorf("systemconfig.cache_ttl 必须大于 0 且不超过 24h")
	}
	return nil
}
