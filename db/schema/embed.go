// Package schema 提供显式数据库初始化 SQL 及应用期望的 Schema 版本。
package schema

import _ "embed"

const CurrentVersion int32 = 1

// InitialSQL 仅供部署方显式执行的 db init 命令使用，serve 不得执行。
//
//go:embed 000001_initial.sql
var InitialSQL string
