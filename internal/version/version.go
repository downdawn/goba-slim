// Package version 提供构建版本信息。
package version

import "fmt"

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// BuildInfo 是不可变的构建元数据快照。
type BuildInfo struct {
	Version   string
	Commit    string
	BuildTime string
}

// Info 返回当前构建信息。
func Info() BuildInfo { return BuildInfo{Version: Version, Commit: Commit, BuildTime: BuildTime} }

// String 返回供 CLI 显示的稳定格式。
func (b BuildInfo) String() string {
	return fmt.Sprintf("%s (commit=%s, built=%s)", b.Version, b.Commit, b.BuildTime)
}
