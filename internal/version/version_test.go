package version

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInfoReturnsBuildValues(t *testing.T) {
	originalVersion, originalCommit, originalBuildTime := Version, Commit, BuildTime
	t.Cleanup(func() { Version, Commit, BuildTime = originalVersion, originalCommit, originalBuildTime })
	Version, Commit, BuildTime = "v1.2.3", "abc123", "2026-07-12T00:00:00Z"

	info := Info()

	require.Equal(t, BuildInfo{Version: "v1.2.3", Commit: "abc123", BuildTime: "2026-07-12T00:00:00Z"}, info)
	require.True(t, strings.Contains(info.String(), "v1.2.3"))
	require.True(t, strings.Contains(info.String(), "abc123"))
}
