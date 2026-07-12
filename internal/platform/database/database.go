// Package database 提供 PostgreSQL 连接、Schema 检查和显式初始化能力。
package database

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	dbgen "github.com/downdawn/goba-slim/db/generated"
	"github.com/downdawn/goba-slim/db/schema"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotStarted       = errors.New("PostgreSQL 尚未启动")
	ErrSchemaMissing    = errors.New("数据库 Schema 尚未初始化")
	ErrSchemaMismatch   = errors.New("数据库 Schema 版本不匹配")
	ErrDatabaseNotEmpty = errors.New("目标数据库不是空数据库")
)

type Status struct {
	ServerVersion string
	SchemaVersion int32
	Expected      int32
	Initialized   bool
}

type Component struct {
	config        *pgxpool.Config
	healthTimeout time.Duration

	mu   sync.RWMutex
	pool *pgxpool.Pool
}

func New(cfg config.DatabaseConfig) (*Component, error) {
	poolConfig, err := newPoolConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Component{config: poolConfig, healthTimeout: cfg.HealthTimeout}, nil
}

func (c *Component) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pool != nil {
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, c.config.Copy())
	if err != nil {
		return fmt.Errorf("创建 PostgreSQL 连接池失败: %w", err)
	}
	if _, err := inspect(ctx, pool, true); err != nil {
		pool.Close()
		return err
	}
	c.pool = pool
	return nil
}

func (c *Component) Stop(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pool != nil {
		c.pool.Close()
		c.pool = nil
	}
	return nil
}

func (c *Component) Health(ctx context.Context) error {
	pool, err := c.currentPool()
	if err != nil {
		return err
	}
	healthCtx, cancel := context.WithTimeout(ctx, c.healthTimeout)
	defer cancel()
	_, err = inspect(healthCtx, pool, true)
	return err
}

func (c *Component) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	pool, err := c.currentPool()
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	return pool.Exec(ctx, sql, arguments...)
}

func (c *Component) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	pool, err := c.currentPool()
	if err != nil {
		return nil, err
	}
	return pool.Query(ctx, sql, args...)
}

func (c *Component) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	pool, err := c.currentPool()
	if err != nil {
		return errorRow{err: err}
	}
	return pool.QueryRow(ctx, sql, args...)
}

func (c *Component) Begin(ctx context.Context) (pgx.Tx, error) {
	pool, err := c.currentPool()
	if err != nil {
		return nil, err
	}
	return pool.Begin(ctx)
}

func (c *Component) currentPool() (*pgxpool.Pool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.pool == nil {
		return nil, ErrNotStarted
	}
	return c.pool, nil
}

type errorRow struct{ err error }

func (r errorRow) Scan(...any) error { return r.err }

func Open(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolConfig, err := newPoolConfig(cfg)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 PostgreSQL 连接池失败: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}
	return pool, nil
}

func Inspect(ctx context.Context, cfg config.DatabaseConfig) (Status, error) {
	pool, err := Open(ctx, cfg)
	if err != nil {
		return Status{}, err
	}
	defer pool.Close()
	return inspect(ctx, pool, false)
}

func Initialize(ctx context.Context, cfg config.DatabaseConfig) error {
	pool, err := Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("开始数据库初始化事务失败: %w", err)
	}
	//nolint:contextcheck // 回滚清理不能依赖可能已取消的初始化 Context。
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext('goba.schema.init'))"); err != nil {
		return fmt.Errorf("锁定数据库初始化失败: %w", err)
	}
	queries := dbgen.New(tx)
	tableCount, err := queries.CountPublicTables(ctx)
	if err != nil {
		return fmt.Errorf("检查数据库表失败: %w", err)
	}
	if tableCount != 0 {
		return ErrDatabaseNotEmpty
	}
	if _, err := tx.Exec(ctx, schema.InitialSQL); err != nil {
		return fmt.Errorf("执行初始化 SQL 失败: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("提交数据库初始化失败: %w", err)
	}
	return nil
}

func inspect(ctx context.Context, pool *pgxpool.Pool, requireSchema bool) (Status, error) {
	if err := pool.Ping(ctx); err != nil {
		return Status{}, fmt.Errorf("PostgreSQL 就绪检查失败: %w", err)
	}
	var versionNumberText string
	var version string
	if err := pool.QueryRow(ctx, "SHOW server_version_num").Scan(&versionNumberText); err != nil {
		return Status{}, fmt.Errorf("读取 PostgreSQL 版本失败: %w", err)
	}
	versionNumber, err := strconv.Atoi(versionNumberText)
	if err != nil {
		return Status{}, fmt.Errorf("读取 PostgreSQL 版本失败")
	}
	if err := pool.QueryRow(ctx, "SHOW server_version").Scan(&version); err != nil {
		return Status{}, fmt.Errorf("读取 PostgreSQL 版本失败: %w", err)
	}
	if versionNumber/10000 < 16 {
		return Status{}, fmt.Errorf("PostgreSQL 版本必须为 16 或更高")
	}
	status := Status{ServerVersion: version, Expected: schema.CurrentVersion}
	migration, err := dbgen.New(pool).GetSchemaVersion(ctx)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			if requireSchema {
				return Status{}, ErrSchemaMissing
			}
			return status, nil
		}
		return Status{}, fmt.Errorf("检查数据库 Schema 版本失败: %w", err)
	}
	status.SchemaVersion = migration.Version
	status.Initialized = true
	if migration.Version != schema.CurrentVersion {
		return status, fmt.Errorf("%w: 当前版本 %d，期望版本 %d", ErrSchemaMismatch, migration.Version, schema.CurrentVersion)
	}
	return status, nil
}

func newPoolConfig(cfg config.DatabaseConfig) (*pgxpool.Config, error) {
	endpoint := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password.Reveal()),
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Path:   cfg.Name,
	}
	query := endpoint.Query()
	query.Set("sslmode", cfg.SSLMode)
	query.Set("connect_timeout", strconv.Itoa(max(1, int(cfg.ConnectTimeout/time.Second))))
	endpoint.RawQuery = query.Encode()
	poolConfig, err := pgxpool.ParseConfig(endpoint.String())
	if err != nil {
		return nil, fmt.Errorf("数据库配置无效")
	}
	poolConfig.MinConns = cfg.MinConnections
	poolConfig.MaxConns = cfg.MaxConnections
	poolConfig.ConnConfig.RuntimeParams["timezone"] = "UTC"
	return poolConfig, nil
}
