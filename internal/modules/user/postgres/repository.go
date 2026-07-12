// Package postgres 实现用户模块的 PostgreSQL 仓储边界。
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	dbgen "github.com/downdawn/goba-slim/db/generated"
	"github.com/downdawn/goba-slim/internal/modules/user"
	"github.com/google/uuid"
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
		return nil, fmt.Errorf("用户 PostgreSQL 数据库不能为空")
	}
	return &Store{database: database, queries: dbgen.New(database)}, nil
}

func (s *Store) WithinTransaction(ctx context.Context, fn func(user.Repository) error) error {
	if fn == nil {
		return fmt.Errorf("事务回调不能为空")
	}
	tx, err := s.database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("开始用户事务失败: %w", err)
	}
	//nolint:contextcheck // 回滚清理不能依赖可能已取消的事务 Context。
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := fn(&repository{queries: dbgen.New(tx)}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("提交用户事务失败: %w", err)
	}
	return nil
}

func (s *Store) Create(ctx context.Context, item user.User) (user.User, error) {
	return (&repository{queries: s.queries}).Create(ctx, item)
}
func (s *Store) GetByID(ctx context.Context, userID uuid.UUID) (user.User, error) {
	return (&repository{queries: s.queries}).GetByID(ctx, userID)
}
func (s *Store) GetByUsername(ctx context.Context, username string) (user.User, error) {
	return (&repository{queries: s.queries}).GetByUsername(ctx, username)
}
func (s *Store) List(ctx context.Context, filter user.ListFilter, limit, offset int32) ([]user.User, int64, error) {
	return (&repository{queries: s.queries}).List(ctx, filter, limit, offset)
}
func (s *Store) UpdateProfile(ctx context.Context, userID uuid.UUID, input user.UpdateProfileInput, now time.Time) (user.User, error) {
	return (&repository{queries: s.queries}).UpdateProfile(ctx, userID, input, now)
}
func (s *Store) SetStatus(ctx context.Context, userID uuid.UUID, status user.Status, now time.Time) (user.User, error) {
	return (&repository{queries: s.queries}).SetStatus(ctx, userID, status, now)
}
func (s *Store) SetSuperuser(ctx context.Context, userID uuid.UUID, enabled bool, now time.Time) (user.User, error) {
	return (&repository{queries: s.queries}).SetSuperuser(ctx, userID, enabled, now)
}
func (s *Store) SetMultipleSessions(ctx context.Context, userID uuid.UUID, enabled bool, now time.Time) (user.User, error) {
	return (&repository{queries: s.queries}).SetMultipleSessions(ctx, userID, enabled, now)
}
func (s *Store) UpdatePassword(ctx context.Context, userID uuid.UUID, hash string, now time.Time) (user.User, error) {
	return (&repository{queries: s.queries}).UpdatePassword(ctx, userID, hash, now)
}
func (s *Store) UpdateLastLogin(ctx context.Context, userID uuid.UUID, now time.Time) error {
	return (&repository{queries: s.queries}).UpdateLastLogin(ctx, userID, now)
}
func (s *Store) LockSuperuserChanges(ctx context.Context) error {
	return (&repository{queries: s.queries}).LockSuperuserChanges(ctx)
}
func (s *Store) CountActiveSuperusers(ctx context.Context) (int64, error) {
	return (&repository{queries: s.queries}).CountActiveSuperusers(ctx)
}

type repository struct{ queries *dbgen.Queries }

func (r *repository) Create(ctx context.Context, item user.User) (user.User, error) {
	result, err := r.queries.CreateUser(ctx, dbgen.CreateUserParams{
		ID: item.ID, Username: item.Username, PasswordHash: item.PasswordHash, DisplayName: item.DisplayName,
		Email: toText(item.Email), AvatarUrl: toText(item.AvatarURL), Status: string(item.Status),
		IsSuperuser: item.IsSuperuser, AllowMultipleSessions: item.AllowMultipleSessions,
		SessionVersion: item.SessionVersion, PasswordChangedAt: toTimestamp(item.PasswordChangedAt),
		CreatedAt: toTimestamp(item.CreatedAt), UpdatedAt: toTimestamp(item.UpdatedAt),
	})
	return mapResult("创建用户", result, err)
}

func (r *repository) GetByID(ctx context.Context, userID uuid.UUID) (user.User, error) {
	result, err := r.queries.GetUserByID(ctx, userID)
	return mapResult("读取用户", result, err)
}

func (r *repository) GetByUsername(ctx context.Context, username string) (user.User, error) {
	result, err := r.queries.GetUserByUsername(ctx, username)
	return mapResult("读取用户", result, err)
}

func (r *repository) List(ctx context.Context, filter user.ListFilter, limit, offset int32) ([]user.User, int64, error) {
	params := dbgen.ListUsersParams{UsernameFilter: filter.Username, StatusFilter: string(filter.Status), PageLimit: limit, PageOffset: offset}
	rows, err := r.queries.ListUsers(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("查询用户列表失败: %w", err)
	}
	total, err := r.queries.CountUsers(ctx, dbgen.CountUsersParams{UsernameFilter: filter.Username, StatusFilter: string(filter.Status)})
	if err != nil {
		return nil, 0, fmt.Errorf("统计用户列表失败: %w", err)
	}
	items := make([]user.User, 0, len(rows))
	for _, row := range rows {
		items = append(items, toUser(row))
	}
	return items, total, nil
}

func (r *repository) UpdateProfile(ctx context.Context, userID uuid.UUID, input user.UpdateProfileInput, now time.Time) (user.User, error) {
	result, err := r.queries.UpdateUserProfile(ctx, dbgen.UpdateUserProfileParams{ID: userID, Username: input.Username, DisplayName: input.DisplayName, Email: toTextValue(input.Email), AvatarUrl: toTextValue(input.AvatarURL), UpdatedAt: toTimestamp(now)})
	return mapResult("更新用户资料", result, err)
}

func (r *repository) SetStatus(ctx context.Context, userID uuid.UUID, status user.Status, now time.Time) (user.User, error) {
	result, err := r.queries.SetUserStatus(ctx, dbgen.SetUserStatusParams{ID: userID, Status: string(status), UpdatedAt: toTimestamp(now)})
	return mapResult("更新用户状态", result, err)
}

func (r *repository) SetSuperuser(ctx context.Context, userID uuid.UUID, enabled bool, now time.Time) (user.User, error) {
	result, err := r.queries.SetUserSuperuser(ctx, dbgen.SetUserSuperuserParams{ID: userID, IsSuperuser: enabled, UpdatedAt: toTimestamp(now)})
	return mapResult("更新超级管理员状态", result, err)
}

func (r *repository) SetMultipleSessions(ctx context.Context, userID uuid.UUID, enabled bool, now time.Time) (user.User, error) {
	result, err := r.queries.SetUserMultipleSessions(ctx, dbgen.SetUserMultipleSessionsParams{ID: userID, AllowMultipleSessions: enabled, UpdatedAt: toTimestamp(now)})
	return mapResult("更新多会话设置", result, err)
}

func (r *repository) UpdatePassword(ctx context.Context, userID uuid.UUID, hash string, now time.Time) (user.User, error) {
	result, err := r.queries.UpdateUserPassword(ctx, dbgen.UpdateUserPasswordParams{ID: userID, PasswordHash: hash, PasswordChangedAt: toTimestamp(now)})
	return mapResult("更新用户密码", result, err)
}

func (r *repository) UpdateLastLogin(ctx context.Context, userID uuid.UUID, now time.Time) error {
	if err := r.queries.UpdateUserLastLogin(ctx, dbgen.UpdateUserLastLoginParams{ID: userID, LastLoginAt: toTimestamp(now)}); err != nil {
		return fmt.Errorf("更新最后登录时间失败: %w", err)
	}
	return nil
}

func (r *repository) LockSuperuserChanges(ctx context.Context) error {
	if err := r.queries.LockSuperuserChanges(ctx); err != nil {
		return fmt.Errorf("锁定超级管理员变更失败: %w", err)
	}
	return nil
}

func (r *repository) CountActiveSuperusers(ctx context.Context) (int64, error) {
	count, err := r.queries.CountActiveSuperusers(ctx)
	if err != nil {
		return 0, fmt.Errorf("统计可用超级管理员失败: %w", err)
	}
	return count, nil
}

func mapResult(operation string, result dbgen.User, err error) (user.User, error) {
	if err == nil {
		return toUser(result), nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return user.User{}, user.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "users_username_unique":
			return user.User{}, user.ErrUsernameConflict
		case "users_email_unique":
			return user.User{}, user.ErrEmailConflict
		}
	}
	return user.User{}, fmt.Errorf("%s失败: %w", operation, err)
}

func toUser(item dbgen.User) user.User {
	return user.User{
		ID: item.ID, Username: item.Username, PasswordHash: item.PasswordHash, DisplayName: item.DisplayName,
		Email: fromText(item.Email), AvatarURL: fromText(item.AvatarUrl), Status: user.Status(item.Status),
		IsSuperuser: item.IsSuperuser, AllowMultipleSessions: item.AllowMultipleSessions,
		SessionVersion: item.SessionVersion, PasswordChangedAt: item.PasswordChangedAt.Time,
		LastLoginAt: fromTimestamp(item.LastLoginAt), CreatedAt: item.CreatedAt.Time, UpdatedAt: item.UpdatedAt.Time,
		ArchivedAt: fromTimestamp(item.ArchivedAt),
	}
}

func toText(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}

func toTextValue(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}

func fromText(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func toTimestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func fromTimestamp(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}

var _ user.Repository = (*Store)(nil)
var _ user.UnitOfWork = (*Store)(nil)
