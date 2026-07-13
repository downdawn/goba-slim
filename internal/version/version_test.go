package version

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInfoReturnsBuildValues(t *testing.T) {
	originalVersion, originalCommit, originalBuildTime, originalDirty := Version, Commit, BuildTime, Dirty
	t.Cleanup(func() {
		Version, Commit, BuildTime, Dirty = originalVersion, originalCommit, originalBuildTime, originalDirty
	})
	Version, Commit, BuildTime = "v1.2.3", "abc123", "2026-07-12T00:00:00Z"
	Dirty = "false"

	info := Info()

	require.Equal(t, "v1.2.3", info.Version)
	require.Equal(t, "abc123", info.Commit)
	require.Equal(t, "2026-07-12T00:00:00Z", info.BuildTime)
	require.Equal(t, "false", info.Dirty)
	require.NotEmpty(t, info.GoVersion)
	require.True(t, strings.Contains(info.String(), "v1.2.3"))
	require.True(t, strings.Contains(info.String(), "abc123"))
}
