// Package postgres 实现动态配置模块的 PostgreSQL 仓储边界。
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	dbgen "github.com/downdawn/goba-slim/db/generated"
	"github.com/downdawn/goba-slim/internal/modules/systemconfig"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type Database interface {
	dbgen.DBTX
	Begin(context.Context) (pgx.Tx, error)
}

type Store struct {
	database Database
	queries  *dbgen.Queries
}

func New(database Database) (*Store, error) {
	if database == nil {
		return nil, fmt.Errorf("动态配置 PostgreSQL 数据库不能为空")
	}
	return &Store{database: database, queries: dbgen.New(database)}, nil
}

func (s *Store) WithinTransaction(ctx context.Context, fn func(systemconfig.Repository) error) error {
	if fn == nil {
		return fmt.Errorf("事务回调不能为空")
	}
	tx, err := s.database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("开始动态配置事务: %w", err)
	}
	//nolint:contextcheck // 回滚清理不能依赖可能已取消的事务 Context。
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := fn(&repository{queries: dbgen.New(tx)}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("提交动态配置事务: %w", err)
	}
	return nil
}

func (s *Store) Create(ctx context.Context, item systemconfig.Config) (systemconfig.Config, error) {
	return (&repository{queries: s.queries}).Create(ctx, item)
}
func (s *Store) Get(ctx context.Context, key string) (systemconfig.Config, error) {
	return (&repository{queries: s.queries}).Get(ctx, key)
}
func (s *Store) List(ctx context.Context) ([]systemconfig.Config, error) {
	return (&repository{queries: s.queries}).List(ctx)
}
func (s *Store) ListPublic(ctx context.Context) ([]systemconfig.Config, error) {
	return (&repository{queries: s.queries}).ListPublic(ctx)
}
func (s *Store) Update(ctx context.Context, item systemconfig.Config) (systemconfig.Config, error) {
	return (&repository{queries: s.queries}).Update(ctx, item)
}
func (s *Store) Delete(ctx context.Context, key string) error {
	return (&repository{queries: s.queries}).Delete(ctx, key)
}

type repository struct{ queries *dbgen.Queries }

func (r *repository) Create(ctx context.Context, item systemconfig.Config) (systemconfig.Config, error) {
	created, err := r.queries.CreateSystemConfig(ctx, dbgen.CreateSystemConfigParams{Key: item.Key, Value: item.Value, ValueType: string(item.ValueType), IsPublic: item.IsPublic, Description: item.Description, CreatedAt: timestamp(item.CreatedAt), UpdatedAt: timestamp(item.UpdatedAt)})
	if err != nil {
		return systemconfig.Config{}, mapError(err)
	}
	return fromRecord(created), nil
}

func (r *repository) Get(ctx context.Context, key string) (systemconfig.Config, error) {
	item, err := r.queries.GetSystemConfig(ctx, key)
	if err != nil {
		return systemconfig.Config{}, mapError(err)
	}
	return fromRecord(item), nil
}

func (r *repository) List(ctx context.Context) ([]systemconfig.Config, error) {
	items, err := r.queries.ListSystemConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询动态配置: %w", err)
	}
	return fromRecords(items), nil
}

func (r *repository) ListPublic(ctx context.Context) ([]systemconfig.Config, error) {
	items, err := r.queries.ListPublicSystemConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询公开动态配置: %w", err)
	}
	return fromRecords(items), nil
}

func (r *repository) Update(ctx context.Context, item systemconfig.Config) (systemconfig.Config, error) {
	updated, err := r.queries.UpdateSystemConfig(ctx, dbgen.UpdateSystemConfigParams{Key: item.Key, Value: item.Value, ValueType: string(item.ValueType), IsPublic: item.IsPublic, Description: item.Description, UpdatedAt: timestamp(item.UpdatedAt)})
	if err != nil {
		return systemconfig.Config{}, mapError(err)
	}
	return fromRecord(updated), nil
}

func (r *repository) Delete(ctx context.Context, key string) error {
	rows, err := r.queries.DeleteSystemConfig(ctx, key)
	if err != nil {
		return fmt.Errorf("删除动态配置: %w", err)
	}
	if rows == 0 {
		return systemconfig.ErrNotFound
	}
	return nil
}

func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return systemconfig.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return systemconfig.ErrConflict
	}
	return fmt.Errorf("动态配置 PostgreSQL 操作失败: %w", err)
}

func fromRecords(records []dbgen.SystemConfig) []systemconfig.Config {
	items := make([]systemconfig.Config, 0, len(records))
	for _, record := range records {
		items = append(items, fromRecord(record))
	}
	return items
}

func fromRecord(record dbgen.SystemConfig) systemconfig.Config {
	return systemconfig.Config{Key: record.Key, Value: append([]byte(nil), record.Value...), ValueType: systemconfig.ValueType(record.ValueType), IsPublic: record.IsPublic, Description: record.Description, CreatedAt: record.CreatedAt.Time.UTC(), UpdatedAt: record.UpdatedAt.Time.UTC()}
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

var _ systemconfig.Repository = (*Store)(nil)
var _ systemconfig.UnitOfWork = (*Store)(nil)
