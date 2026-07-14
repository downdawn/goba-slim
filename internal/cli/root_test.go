package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/downdawn/goba-slim/internal/app"
	"github.com/downdawn/goba-slim/internal/modules/user"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/platform/database"
	"github.com/downdawn/goba-slim/internal/version"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestVersionPrintsBuildInfo(t *testing.T) {
	oldVersion, oldCommit, oldBuildTime, oldDirty := version.Version, version.Commit, version.BuildTime, version.Dirty
	t.Cleanup(func() {
		version.Version, version.Commit, version.BuildTime, version.Dirty = oldVersion, oldCommit, oldBuildTime, oldDirty
	})
	version.Version = "v0.1.0"
	version.Commit = "abc123"
	version.BuildTime = "2026-07-11T00:00:00Z"
	version.Dirty = "false"

	cmd, output := newTestRoot(t)
	cmd.SetArgs([]string{"version"})

	require.NoError(t, cmd.ExecuteContext(t.Context()))
	require.Contains(t, output.String(), "v0.1.0 (commit=abc123, built=2026-07-11T00:00:00Z, dirty=false, go=go")
}

func TestConfigCheckAcceptsValidConfiguration(t *testing.T) {
	cmd, output := newTestRoot(t)
	cmd.SetArgs([]string{"config", "check", "--config", testConfigPath(t, "server:\n  port: 9000\n")})

	require.NoError(t, cmd.ExecuteContext(t.Context()))
	require.Contains(t, output.String(), "配置检查通过")
}

func TestConfigCheckLoadsRepositoryExample(t *testing.T) {
	cmd, output := newTestRoot(t)
	cmd.SetArgs([]string{"config", "check", "--config", repositoryExampleConfigPath(t)})

	require.NoError(t, cmd.ExecuteContext(t.Context()))
	require.Contains(t, output.String(), "配置检查通过")
}

func TestConfigCheckRejectsInvalidConfiguration(t *testing.T) {
	cmd, _ := newTestRoot(t)
	cmd.SetArgs([]string{"config", "check", "--config", testConfigPath(t, "server:\n  port: 0\n")})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	require.ErrorContains(t, err, "server.port")
}

func TestConfigPrintRedactsSecrets(t *testing.T) {
	cmd, output := newTestRoot(t)
	cmd.SetArgs([]string{"config", "print", "--config", testConfigPath(t, "database:\n  password: database-secret\nauth:\n  private_key: private-secret\n"), "--redact"})

	require.NoError(t, cmd.ExecuteContext(t.Context()))
	require.NotContains(t, output.String(), "private-secret")
	require.NotContains(t, output.String(), "database-secret")
	require.Contains(t, output.String(), "[REDACTED]")
}

func TestConfigPrintDoesNotExposeVerificationPublicKeyContents(t *testing.T) {
	directory := t.TempDir()
	publicKeyPath := filepath.Join(directory, "old-public.pem")
	require.NoError(t, os.WriteFile(publicKeyPath, []byte("public-key-content"), 0o600))
	configPath := filepath.Join(directory, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("auth:\n  verification_key_files:\n    old: "+publicKeyPath+"\n"), 0o600))
	cmd, output := newTestRoot(t)
	cmd.SetArgs([]string{"config", "print", "--config", configPath, "--redact"})

	require.NoError(t, cmd.ExecuteContext(t.Context()))
	var printed redactedConfig
	require.NoError(t, json.Unmarshal(output.Bytes(), &printed))
	require.Equal(t, publicKeyPath, printed.Auth.VerificationKeyFiles["old"])
	require.NotContains(t, output.String(), "public-key-content")
}

func TestDatabaseStatusReportsUninitializedSchema(t *testing.T) {
	output := new(bytes.Buffer)
	cmd := NewRoot(Dependencies{
		Load: config.Load,
		DBStatus: func(context.Context, config.Config) (database.Status, error) {
			return database.Status{ServerVersion: "17.2", Expected: 1}, nil
		},
	})
	cmd.SetOut(output)
	cmd.SetArgs([]string{"db", "status", "--config", testConfigPath(t, "")})

	require.NoError(t, cmd.ExecuteContext(t.Context()))
	require.Contains(t, output.String(), "Schema 尚未初始化")
}

func TestDatabaseMigrateReportsAppliedMigrations(t *testing.T) {
	called := false
	cmd := NewRoot(Dependencies{
		Load: config.Load,
		DBMigrate: func(context.Context, config.Config) (database.MigrationResult, error) {
			called = true
			return database.MigrationResult{Previous: 0, Current: 1, Applied: 1}, nil
		},
	})
	output := new(bytes.Buffer)
	cmd.SetOut(output)
	cmd.SetArgs([]string{"db", "migrate", "--config", testConfigPath(t, "")})

	require.NoError(t, cmd.ExecuteContext(t.Context()))
	require.True(t, called)
	require.Contains(t, output.String(), "版本 0 -> 1")
}

func TestCreateAdminReadsPasswordFileWithoutPrintingPassword(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "admin-password")
	require.NoError(t, os.WriteFile(passwordFile, []byte("ValidPassword9\n"), 0o600))
	output := new(bytes.Buffer)
	var captured user.CreateInput
	cmd := NewRoot(Dependencies{
		Load: config.Load,
		AddAdmin: func(_ context.Context, _ config.Config, input user.CreateInput) (user.User, error) {
			captured = input
			return user.User{ID: uuid.MustParse("019befd7-98d0-7000-8000-000000000003"), Username: input.Username}, nil
		},
	})
	cmd.SetOut(output)
	cmd.SetArgs([]string{"user", "create-admin", "--username", "admin", "--password-file", passwordFile, "--config", testConfigPath(t, "")})

	require.NoError(t, cmd.ExecuteContext(t.Context()))
	require.Equal(t, "admin", captured.Username)
	require.Equal(t, len("ValidPassword9"), len(captured.Password))
	require.NotContains(t, output.String(), "ValidPassword")
}

func TestConfigPrintRequiresRedact(t *testing.T) {
	cmd, _ := newTestRoot(t)
	cmd.SetArgs([]string{"config", "print", "--config", testConfigPath(t, "auth:\n  private_key: private-secret\n")})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	require.ErrorContains(t, err, "--redact")
}

func TestUnknownCommandReturnsError(t *testing.T) {
	cmd, _ := newTestRoot(t)
	cmd.SetArgs([]string{"unknown"})

	require.Error(t, cmd.ExecuteContext(t.Context()))
}

func TestHealthcheckUsesInjectedProbe(t *testing.T) {
	called := false
	cmd := NewRoot(Dependencies{Probe: func(_ context.Context, url string) error {
		called = true
		require.Equal(t, "http://127.0.0.1:8000/readyz", url)
		return nil
	}})
	cmd.SetArgs([]string{"healthcheck"})

	require.NoError(t, cmd.ExecuteContext(t.Context()))
	require.True(t, called)
}

func TestDoctorPrintsSafeChecksAndReturnsFailure(t *testing.T) {
	cmd := NewRoot(Dependencies{
		Load: config.Load,
		Doctor: func(context.Context, config.Config) app.DiagnosticReport {
			return app.DiagnosticReport{Checks: []app.DiagnosticCheck{
				{Name: "auth", OK: true, Message: "Ed25519 密钥可用"},
				{Name: "postgresql", Message: "PostgreSQL 不可用或 Schema 版本不匹配"},
			}}
		},
	})
	output := new(bytes.Buffer)
	cmd.SetOut(output)
	cmd.SetArgs([]string{"doctor", "--config", testConfigPath(t, "")})

	err := cmd.ExecuteContext(t.Context())
	require.ErrorContains(t, err, "doctor 检查未通过")
	require.Contains(t, output.String(), "OK")
	require.Contains(t, output.String(), "FAIL")
	require.NotContains(t, output.String(), "password")
}

func newTestRoot(t *testing.T) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	output := new(bytes.Buffer)
	cmd := NewRoot(Dependencies{
		Load: func(ctx context.Context, options config.Options) (config.Config, error) {
			return config.Load(ctx, options)
		},
	})
	cmd.SetOut(output)
	cmd.SetErr(new(bytes.Buffer))
	return cmd, output
}

func repositoryExampleConfigPath(t *testing.T) string {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "无法定位 CLI 测试文件")
	return filepath.Join(filepath.Dir(sourceFile), "..", "..", "configs", "config.example.yaml")
}

func testConfigPath(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(strings.TrimSpace(contents)+"\n"), 0o600))
	return path
}
