package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadMergesDefaultsYAMLAndEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server:\n  port: 9001\nauth:\n  private_key: yaml-secret\n"), 0o600))
	t.Setenv("GOBA_SERVER_PORT", "9002")

	cfg, err := Load(context.Background(), Options{File: path})

	require.NoError(t, err)
	require.Equal(t, 9002, cfg.Server.Port)
	require.Equal(t, "yaml-secret", cfg.Auth.PrivateKey.Reveal())
	require.Equal(t, 15*time.Second, cfg.Server.ReadTimeout)
}

func TestValidateRejectsUnsafeCorsWildcardWithCredentials(t *testing.T) {
	cfg := Default()
	cfg.CORS.AllowOrigins = []string{"*"}
	cfg.CORS.AllowCredentials = true

	require.ErrorContains(t, cfg.Validate(), "cors.allow_origins")
}

func TestLoadReadsSecretFromFile(t *testing.T) {
	secretFile := filepath.Join(t.TempDir(), "private.key")
	require.NoError(t, os.WriteFile(secretFile, []byte("secret-from-file\n"), 0o600))
	t.Setenv("GOBA_AUTH_PRIVATE_KEY_FILE", secretFile)

	cfg, err := Load(t.Context(), Options{EnvironmentPrefix: "GOBA_"})

	require.NoError(t, err)
	require.Equal(t, "secret-from-file", cfg.Auth.PrivateKey.Reveal())
}

func TestLoadRejectsAmbiguousSecretSources(t *testing.T) {
	t.Setenv("GOBA_AUTH_PRIVATE_KEY", "plain-secret")
	t.Setenv("GOBA_AUTH_PRIVATE_KEY_FILE", filepath.Join(t.TempDir(), "private.key"))

	_, err := Load(t.Context(), Options{EnvironmentPrefix: "GOBA_"})

	require.ErrorContains(t, err, "GOBA_AUTH_PRIVATE_KEY")
	require.NotContains(t, err.Error(), "plain-secret")
}

func TestLoadDotEnvIsOptIn(t *testing.T) {
	directory := t.TempDir()
	t.Chdir(directory)
	require.NoError(t, os.WriteFile(filepath.Join(directory, ".env"), []byte("GOBA_SERVER_PORT=9001\n"), 0o600))

	withoutDotEnv, err := Load(t.Context(), Options{})
	require.NoError(t, err)
	withDotEnv, err := Load(t.Context(), Options{LoadDotEnv: true})

	require.NoError(t, err)
	require.Equal(t, 8000, withoutDotEnv.Server.Port)
	require.Equal(t, 9001, withDotEnv.Server.Port)
}

func TestLoadAppliesEnvironmentOverridesForAllConfigurationSections(t *testing.T) {
	t.Setenv("GOBA_APP_DEBUG", "true")
	t.Setenv("GOBA_APP_DOCS_ENABLED", "false")
	t.Setenv("GOBA_SERVER_READ_TIMEOUT", "20s")
	t.Setenv("GOBA_SERVER_TRUSTED_PROXIES", "10.0.0.0/8,192.168.0.0/16")
	t.Setenv("GOBA_CORS_ALLOW_ORIGINS", "https://app.example.com,https://admin.example.com")
	t.Setenv("GOBA_CORS_ALLOW_CREDENTIALS", "true")
	t.Setenv("GOBA_AUTH_ISSUER", "goba")
	t.Setenv("GOBA_AUTH_ACCESS_TOKEN_TTL", "30m")
	t.Setenv("GOBA_AUTH_REFRESH_TOKEN_TTL", "72h")
	t.Setenv("GOBA_LOG_LEVEL", "debug")
	t.Setenv("GOBA_LOG_FORMAT", "text")
	t.Setenv("GOBA_MODULES_FILE", "true")
	t.Setenv("GOBA_MODULES_SYSTEMCONFIG", "true")

	cfg, err := Load(t.Context(), Options{EnvironmentPrefix: "GOBA_"})

	require.NoError(t, err)
	require.True(t, cfg.App.Debug)
	require.False(t, cfg.App.DocsEnabled)
	require.Equal(t, 20*time.Second, cfg.Server.ReadTimeout)
	require.Equal(t, []string{"10.0.0.0/8", "192.168.0.0/16"}, cfg.Server.TrustedProxies)
	require.Equal(t, []string{"https://app.example.com", "https://admin.example.com"}, cfg.CORS.AllowOrigins)
	require.True(t, cfg.CORS.AllowCredentials)
	require.Equal(t, "goba", cfg.Auth.Issuer)
	require.Equal(t, 30*time.Minute, cfg.Auth.AccessTokenTTL)
	require.Equal(t, 72*time.Hour, cfg.Auth.RefreshTokenTTL)
	require.Equal(t, "debug", cfg.Log.Level)
	require.Equal(t, "text", cfg.Log.Format)
	require.True(t, cfg.Modules.File)
	require.True(t, cfg.Modules.SystemConfig)
}

func TestValidateRejectsUnsafeConfigurations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		field  string
	}{
		{name: "invalid environment", mutate: func(cfg *Config) { cfg.App.Environment = "staging" }, field: "app.environment"},
		{name: "nonpositive timeout", mutate: func(cfg *Config) { cfg.Server.ReadTimeout = 0 }, field: "server.read_timeout"},
		{name: "inverted token TTL", mutate: func(cfg *Config) { cfg.Auth.AccessTokenTTL = cfg.Auth.RefreshTokenTTL }, field: "auth.access_token_ttl"},
		{name: "invalid trusted proxy", mutate: func(cfg *Config) { cfg.Server.TrustedProxies = []string{"invalid"} }, field: "server.trusted_proxies"},
		{name: "production debug", mutate: func(cfg *Config) { cfg.App.Environment = "production"; cfg.App.Debug = true }, field: "app.debug"},
		{name: "production documentation", mutate: func(cfg *Config) { cfg.App.Environment = "production"; cfg.App.DocsEnabled = true }, field: "app.docs_enabled"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := Default()
			test.mutate(&cfg)

			require.ErrorContains(t, cfg.Validate(), test.field)
		})
	}
}

func TestSecretStringIsRedacted(t *testing.T) {
	require.Equal(t, "[REDACTED]", NewSecret("actual-secret").String())
}
