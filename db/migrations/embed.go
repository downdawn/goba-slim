// Package migrations 提供由二进制嵌入的 PostgreSQL Schema 迁移。
package migrations

import "embed"

// Files 包含按顺序编号的向前迁移 SQL。
//
//go:embed *.sql
var Files embed.FS
