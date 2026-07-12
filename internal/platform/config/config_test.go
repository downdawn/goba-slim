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

func TestSecretStringIsRedacted(t *testing.T) {
	require.Equal(t, "[REDACTED]", NewSecret("actual-secret").String())
}
