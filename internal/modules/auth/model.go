// Package auth 提供认证、Token 与会话应用能力。
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/downdawn/goba-slim/internal/modules/user"
	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrUserDisabled       = errors.New("用户已停用")
	ErrInvalidToken       = errors.New("token 无效或已过期")
	ErrRefreshReuse       = errors.New("Refresh Token 已被重复使用")
	ErrRateLimited        = errors.New("登录尝试过于频繁")
)

type Session struct {
	ID            uuid.UUID `json:"id"`
	FamilyID      uuid.UUID `json:"family_id"`
	UserID        uuid.UUID `json:"user_id"`
	UserVersion   int64     `json:"user_version"`
	CurrentDigest string    `json:"current_digest"`
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	SessionID    uuid.UUID
	User         user.User
}

type Identity struct {
	User      user.User
	SessionID uuid.UUID
}

type UserService interface {
	VerifyCredentials(context.Context, string, string) (user.User, error)
	GetByID(context.Context, uuid.UUID) (user.User, error)
	RecordLogin(context.Context, uuid.UUID) error
}

type SessionStore interface {
	Create(context.Context, Session, time.Duration, bool) error
	Get(context.Context, uuid.UUID) (Session, error)
	Rotate(context.Context, string, string, time.Time, time.Duration) (Session, error)
	Revoke(context.Context, uuid.UUID) error
	RevokeUser(context.Context, uuid.UUID) error
}

type RateLimiter interface {
	Allow(context.Context, string, int, time.Duration) (bool, error)
}
