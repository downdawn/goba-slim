// Package redis 实现认证会话与限流的 Redis 适配器。
package redis

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/downdawn/goba-slim/internal/modules/auth"
	"github.com/google/uuid"
	redisclient "github.com/redis/go-redis/v9"
)

//go:embed rotate.lua
var rotateLua string

type Store struct {
	client        redisclient.UniversalClient
	sessionPrefix string
	refreshPrefix string
	userPrefix    string
	limitPrefix   string
	rotateScript  *redisclient.Script
}

func New(client redisclient.UniversalClient, environment string) (*Store, error) {
	if client == nil || environment == "" {
		return nil, fmt.Errorf("认证 Redis 依赖不能为空")
	}
	prefix := "goba:" + environment + ":auth:"
	return &Store{client: client, sessionPrefix: prefix + "session:", refreshPrefix: prefix + "refresh:", userPrefix: prefix + "user:", limitPrefix: prefix + "limit:", rotateScript: redisclient.NewScript(rotateLua)}, nil
}

func (s *Store) Create(ctx context.Context, session auth.Session, ttl time.Duration, allowMultiple bool) error {
	if !allowMultiple {
		if err := s.RevokeUser(ctx, session.UserID); err != nil {
			return err
		}
	}
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("编码认证会话: %w", err)
	}
	tokenJSON, err := json.Marshal(refreshRecord{SessionID: session.ID, FamilyID: session.FamilyID, UserID: session.UserID, State: "active"})
	if err != nil {
		return fmt.Errorf("编码 Refresh Token 记录: %w", err)
	}
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, s.sessionPrefix+session.ID.String(), sessionJSON, ttl)
	pipe.Set(ctx, s.refreshPrefix+session.CurrentDigest, tokenJSON, ttl)
	pipe.SAdd(ctx, s.userPrefix+session.UserID.String(), session.ID.String())
	pipe.Expire(ctx, s.userPrefix+session.UserID.String(), ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("保存认证会话: %w", err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, sessionID uuid.UUID) (auth.Session, error) {
	value, err := s.client.Get(ctx, s.sessionPrefix+sessionID.String()).Bytes()
	if errors.Is(err, redisclient.Nil) {
		return auth.Session{}, auth.ErrInvalidToken
	}
	if err != nil {
		return auth.Session{}, fmt.Errorf("读取认证会话: %w", err)
	}
	var session auth.Session
	if err := json.Unmarshal(value, &session); err != nil {
		return auth.Session{}, fmt.Errorf("解析认证会话: %w", err)
	}
	return session, nil
}

func (s *Store) Rotate(ctx context.Context, oldDigest, newDigest string, now time.Time, ttl time.Duration) (auth.Session, error) {
	expiresAt := now.Add(ttl).UTC().Format(time.RFC3339Nano)
	result, err := s.rotateScript.Run(ctx, s.client, nil,
		s.refreshPrefix, oldDigest, s.sessionPrefix, s.userPrefix, newDigest, now.UTC().Format(time.RFC3339Nano), strconv.FormatInt(ttl.Milliseconds(), 10), expiresAt,
	).Slice()
	if err != nil {
		return auth.Session{}, fmt.Errorf("轮换 Refresh Token: %w", err)
	}
	if len(result) != 2 {
		return auth.Session{}, auth.ErrInvalidToken
	}
	code, err := toInt64(result[0])
	if err != nil {
		return auth.Session{}, err
	}
	if code == -1 {
		return auth.Session{}, auth.ErrRefreshReuse
	}
	if code != 1 {
		return auth.Session{}, auth.ErrInvalidToken
	}
	raw, ok := result[1].(string)
	if !ok {
		return auth.Session{}, auth.ErrInvalidToken
	}
	var session auth.Session
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return auth.Session{}, fmt.Errorf("解析轮换会话: %w", err)
	}
	return session, nil
}

func (s *Store) Revoke(ctx context.Context, sessionID uuid.UUID) error {
	session, err := s.Get(ctx, sessionID)
	if errors.Is(err, auth.ErrInvalidToken) {
		return nil
	}
	if err != nil {
		return err
	}
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, s.sessionPrefix+session.ID.String(), s.refreshPrefix+session.CurrentDigest)
	pipe.SRem(ctx, s.userPrefix+session.UserID.String(), session.ID.String())
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("撤销认证会话: %w", err)
	}
	return nil
}

func (s *Store) RevokeUser(ctx context.Context, userID uuid.UUID) error {
	key := s.userPrefix + userID.String()
	ids, err := s.client.SMembers(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("读取用户会话索引: %w", err)
	}
	for _, value := range ids {
		sessionID, parseErr := uuid.Parse(value)
		if parseErr != nil {
			continue
		}
		if err := s.Revoke(ctx, sessionID); err != nil {
			return err
		}
	}
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("删除用户会话索引: %w", err)
	}
	return nil
}

func (s *Store) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	redisKey := s.limitPrefix + key
	count, err := s.client.Incr(ctx, redisKey).Result()
	if err != nil {
		return false, fmt.Errorf("更新登录限流: %w", err)
	}
	if count == 1 {
		if err := s.client.Expire(ctx, redisKey, window).Err(); err != nil {
			return false, fmt.Errorf("设置登录限流过期时间: %w", err)
		}
	}
	return count <= int64(limit), nil
}

type refreshRecord struct {
	SessionID uuid.UUID `json:"session_id"`
	FamilyID  uuid.UUID `json:"family_id"`
	UserID    uuid.UUID `json:"user_id"`
	State     string    `json:"state"`
}

func toInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err == nil {
			return parsed, nil
		}
	}
	return 0, fmt.Errorf("redis Lua 返回无效状态")
}

var _ auth.SessionStore = (*Store)(nil)
var _ auth.RateLimiter = (*Store)(nil)
