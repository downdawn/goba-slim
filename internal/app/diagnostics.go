package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/downdawn/goba-slim/internal/modules/auth"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/platform/database"
	"github.com/downdawn/goba-slim/internal/platform/redisstore"
)

type DiagnosticCheck struct {
	Name    string
	OK      bool
	Message string
}

type DiagnosticReport struct {
	Checks []DiagnosticCheck
}

func (r DiagnosticReport) OK() bool {
	for _, check := range r.Checks {
		if !check.OK {
			return false
		}
	}
	return true
}

func Diagnose(ctx context.Context, cfg config.Config) DiagnosticReport {
	report := DiagnosticReport{Checks: make([]DiagnosticCheck, 0, 4)}
	if _, err := auth.NewTokens(cfg.Auth); err != nil {
		report.Checks = append(report.Checks, DiagnosticCheck{Name: "auth", Message: "Ed25519 签名或验证密钥无效"})
	} else {
		report.Checks = append(report.Checks, DiagnosticCheck{Name: "auth", OK: true, Message: "Ed25519 密钥可用"})
	}

	status, err := database.Inspect(ctx, cfg.Database)
	if err != nil {
		report.Checks = append(report.Checks, DiagnosticCheck{Name: "postgresql", Message: "PostgreSQL 不可用或 Schema 版本不匹配"})
	} else if !status.Initialized {
		report.Checks = append(report.Checks, DiagnosticCheck{Name: "postgresql", Message: "PostgreSQL 可连接，但 Schema 尚未初始化"})
	} else {
		report.Checks = append(report.Checks, DiagnosticCheck{Name: "postgresql", OK: true, Message: fmt.Sprintf("PostgreSQL 可用，Schema 版本 %d", status.SchemaVersion)})
	}

	redisComponent := redisstore.New(cfg.Redis)
	if err := redisComponent.Start(ctx); err != nil {
		report.Checks = append(report.Checks, DiagnosticCheck{Name: "redis", Message: "Redis 不可用"})
	} else {
		report.Checks = append(report.Checks, DiagnosticCheck{Name: "redis", OK: true, Message: "Redis 可用"})
	}
	//nolint:contextcheck // 诊断请求可能已取消，客户端关闭必须使用独立清理 Context。
	_ = redisComponent.Stop(context.Background())

	if cfg.Modules.File {
		report.Checks = append(report.Checks, checkFileStorage(cfg.File.StoragePath))
	}
	return report
}

func checkFileStorage(storagePath string) DiagnosticCheck {
	check := DiagnosticCheck{Name: "file-storage"}
	info, err := os.Stat(storagePath)
	if err != nil || !info.IsDir() {
		check.Message = "文件模块已启用，但存储目录不存在或不是目录"
		return check
	}
	temporary, err := os.CreateTemp(storagePath, ".goba-doctor-*")
	if err != nil {
		check.Message = "文件模块已启用，但存储目录不可写"
		return check
	}
	name := temporary.Name()
	if closeErr := temporary.Close(); closeErr != nil {
		_ = os.Remove(name)
		check.Message = "文件模块存储目录写入检查失败"
		return check
	}
	if removeErr := os.Remove(name); removeErr != nil {
		check.Message = "文件模块存储目录临时文件清理失败"
		return check
	}
	check.OK = true
	check.Message = "文件存储目录可写: " + filepath.Clean(storagePath)
	return check
}
