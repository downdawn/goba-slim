package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeneratePrivateKeyCreatesPKCS8Ed25519AndRefusesOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.pem")
	require.NoError(t, generatePrivateKey(path))

	content := readTestFile(t, path)
	block, rest := pem.Decode(content)
	require.NotNil(t, block)
	require.Empty(t, rest)
	require.Equal(t, "PRIVATE KEY", block.Type)
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)
	_, ok := parsed.(ed25519.PrivateKey)
	require.True(t, ok)

	require.ErrorContains(t, generatePrivateKey(path), "拒绝覆盖")
	unchanged := readTestFile(t, path)
	require.Equal(t, content, unchanged)
}

func TestSetupCreatesMissingFilesAndPreservesExistingFiles(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "configs"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "configs", "config.example.yaml"), []byte("app:\n  environment: development\n"), 0o600))

	var output bytes.Buffer
	require.NoError(t, setup(root, &output))

	envPath := filepath.Join(root, ".env")
	envContent := readTestFile(t, envPath)
	require.Contains(t, string(envContent), "GOBA_DATABASE_PASSWORD=")
	require.Contains(t, string(envContent), "GOBA_REDIS_PASSWORD=")
	require.Contains(t, string(envContent), "GOBA_AUTH_PRIVATE_KEY_FILE=configs/auth-private.local.pem")
	require.NotContains(t, output.String(), strings.TrimSpace(string(envContent)))

	keyPath := filepath.Join(root, filepath.FromSlash(defaultPrivateKeyPath))
	keyContent := readTestFile(t, keyPath)
	configContent := readTestFile(t, filepath.Join(root, "configs", "config.local.yaml"))

	output.Reset()
	require.NoError(t, setup(root, &output))
	require.Contains(t, output.String(), "保留")
	require.FileExists(t, envPath)
	actualKey := readTestFile(t, keyPath)
	require.Equal(t, keyContent, actualKey)
	actualConfig := readTestFile(t, filepath.Join(root, "configs", "config.local.yaml"))
	require.Equal(t, configContent, actualConfig)
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	// #nosec G304 -- 测试只读取自身临时目录中的文件。
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}
