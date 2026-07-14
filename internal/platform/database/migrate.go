package database

import (
	"context"
	"fmt"

	"github.com/downdawn/goba-slim/db/migrations"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/tern/v2/migrate"
)

const versionTable = "public.goba_schema_version"

type MigrationResult struct {
	Previous int32
	Current  int32
	Applied  int32
}

func Migrate(ctx context.Context, cfg config.DatabaseConfig) (MigrationResult, error) {
	pool, err := Open(ctx, cfg)
	if err != nil {
		return MigrationResult{}, err
	}
	defer pool.Close()
	if _, err := serverVersion(ctx, pool); err != nil {
		return MigrationResult{}, err
	}

	connection, err := pool.Acquire(ctx)
	if err != nil {
		return MigrationResult{}, fmt.Errorf("获取迁移数据库连接失败: %w", err)
	}
	defer connection.Release()
	if err := lockMigration(ctx, connection.Conn()); err != nil {
		return MigrationResult{}, err
	}
	defer func() { _ = unlockMigration(context.WithoutCancel(ctx), connection.Conn()) }()

	managed, err := versionTableExists(ctx, connection.Conn())
	if err != nil {
		return MigrationResult{}, err
	}
	if !managed {
		nonEmpty, countErr := hasPublicTables(ctx, connection.Conn())
		if countErr != nil {
			return MigrationResult{}, countErr
		}
		if nonEmpty {
			return MigrationResult{}, ErrDatabaseNotEmpty
		}
	}

	migrator, err := newMigrator(ctx, connection.Conn())
	if err != nil {
		return MigrationResult{}, err
	}
	previous, err := migrator.GetCurrentVersion(ctx)
	if err != nil {
		return MigrationResult{}, fmt.Errorf("读取迁移版本失败: %w", err)
	}
	if err := migrator.Migrate(ctx); err != nil {
		return MigrationResult{}, fmt.Errorf("执行数据库迁移失败: %w", err)
	}
	current, err := migrator.GetCurrentVersion(ctx)
	if err != nil {
		return MigrationResult{}, fmt.Errorf("读取迁移结果失败: %w", err)
	}
	return MigrationResult{Previous: previous, Current: current, Applied: current - previous}, nil
}

func expectedVersion(ctx context.Context) (int32, error) {
	migrator, err := migrate.NewMigrator(ctx, nil, versionTable)
	if err != nil {
		return 0, fmt.Errorf("创建迁移器失败: %w", err)
	}
	if err := migrator.LoadMigrations(migrations.Files); err != nil {
		return 0, fmt.Errorf("加载数据库迁移失败: %w", err)
	}
	if len(migrator.Migrations) > int(^uint32(0)>>1) {
		return 0, fmt.Errorf("迁移数量超过版本字段上限")
	}
	// #nosec G115 -- 已显式检查 int32 上界。
	return int32(len(migrator.Migrations)), nil
}

func newMigrator(ctx context.Context, connection *pgx.Conn) (*migrate.Migrator, error) {
	migrator, err := migrate.NewMigrator(ctx, connection, versionTable)
	if err != nil {
		return nil, fmt.Errorf("创建迁移器失败: %w", err)
	}
	if err := migrator.LoadMigrations(migrations.Files); err != nil {
		return nil, fmt.Errorf("加载数据库迁移失败: %w", err)
	}
	return migrator, nil
}

func hasPublicTables(ctx context.Context, connection *pgx.Conn) (bool, error) {
	var count int
	if err := connection.QueryRow(ctx, "SELECT count(*) FROM pg_catalog.pg_tables WHERE schemaname = 'public'").Scan(&count); err != nil {
		return false, fmt.Errorf("检查目标数据库是否为空失败: %w", err)
	}
	return count > 0, nil
}

func lockMigration(ctx context.Context, connection *pgx.Conn) error {
	if _, err := connection.Exec(ctx, "SELECT pg_advisory_lock(hashtext('goba.schema.migrate'))"); err != nil {
		return fmt.Errorf("锁定数据库迁移失败: %w", err)
	}
	return nil
}

func unlockMigration(ctx context.Context, connection *pgx.Conn) error {
	_, err := connection.Exec(ctx, "SELECT pg_advisory_unlock(hashtext('goba.schema.migrate'))")
	return err
}

var _ rowQuerier = (*pgxpool.Pool)(nil)
