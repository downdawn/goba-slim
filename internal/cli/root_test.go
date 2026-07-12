package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/version"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestVersionPrintsBuildInfo(t *testing.T) {
	oldVersion, oldCommit, oldBuildTime := version.Version, version.Commit, version.BuildTime
	t.Cleanup(func() {
		version.Version, version.Commit, version.BuildTime = oldVersion, oldCommit, oldBuildTime
	})
	version.Version = "v0.1.0"
	version.Commit = "abc123"
	version.BuildTime = "2026-07-11T00:00:00Z"

	cmd, output := newTestRoot(t)
	cmd.SetArgs([]string{"version"})

	require.NoError(t, cmd.ExecuteContext(t.Context()))
	require.Equal(t, "v0.1.0 (commit=abc123, built=2026-07-11T00:00:00Z)\n", output.String())
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
	cmd.SetArgs([]string{"config", "print", "--config", testConfigPath(t, "auth:\n  private_key: private-secret\n"), "--redact"})

	require.NoError(t, cmd.ExecuteContext(t.Context()))
	require.NotContains(t, output.String(), "private-secret")
	require.Contains(t, output.String(), "[REDACTED]")
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
