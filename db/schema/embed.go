// Package schema 提供显式数据库初始化 SQL 及应用期望的 Schema 版本。
package schema

import _ "embed"

const CurrentVersion int32 = 1

// CurrentPublicTables 是当前 Schema 应包含的 public 表，用于拒绝在未知数据库上执行初始化。
var CurrentPublicTables = []string{"schema_migrations", "users"}

// InitialSQL 仅供部署方显式执行的 db init 命令使用，serve 不得执行。
//
//go:embed 000001_initial.sql
var InitialSQL string
