package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/downdawn/goba-slim/internal/modules/user"
	"github.com/downdawn/goba-slim/internal/shared/clock"
	"github.com/google/uuid"
)

type Service struct {
	users       UserService
	sessions    SessionStore
	limiter     RateLimiter
	tokens      *Tokens
	clock       clock.Clock
	refreshTTL  time.Duration
	maxAttempts int
	loginWindow time.Duration
}

func NewService(users UserService, sessions SessionStore, limiter RateLimiter, tokens *Tokens, businessClock clock.Clock, refreshTTL time.Duration, maxAttempts int, loginWindow time.Duration) (*Service, error) {
	if users == nil || sessions == nil || limiter == nil || tokens == nil || businessClock == nil {
		return nil, fmt.Errorf("认证服务依赖不能为空")
	}
	return &Service{users: users, sessions: sessions, limiter: limiter, tokens: tokens, clock: businessClock, refreshTTL: refreshTTL, maxAttempts: maxAttempts, loginWindow: loginWindow}, nil
}

func (s *Service) Login(ctx context.Context, username, password, clientKey string) (TokenPair, error) {
	normalized := strings.ToLower(strings.TrimSpace(username))
	clientKey = strings.TrimSpace(clientKey)
	if clientKey == "" {
		clientKey = "unknown"
	}
	for _, key := range []string{"ip:" + clientKey, "account:" + normalized} {
		allowed, err := s.limiter.Allow(ctx, key, s.maxAttempts, s.loginWindow)
		if err != nil {
			return TokenPair{}, fmt.Errorf("%w: 登录限流: %w", ErrUnavailable, err)
		}
		if !allowed {
			return TokenPair{}, ErrRateLimited
		}
	}
	account, err := s.users.VerifyCredentials(ctx, normalized, password)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) || errors.Is(err, user.ErrPasswordMismatch) {
			return TokenPair{}, ErrInvalidCredentials
		}
		return TokenPair{}, err
	}
	if account.Status != user.StatusActive {
		return TokenPair{}, ErrUserDisabled
	}
	now := s.clock.Now().UTC()
	sessionID, familyID := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	refresh, digest, err := newRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}
	session := Session{ID: sessionID, FamilyID: familyID, UserID: account.ID, UserVersion: account.SessionVersion, CurrentDigest: digest, CreatedAt: now, ExpiresAt: now.Add(s.refreshTTL)}
	if err := s.sessions.Create(ctx, session, s.refreshTTL, account.AllowMultipleSessions); err != nil {
		return TokenPair{}, fmt.Errorf("%w: 创建认证会话: %w", ErrUnavailable, err)
	}
	access, expiresAt, err := s.tokens.Sign(account.ID, sessionID, account.SessionVersion, now)
	if err != nil {
		_ = s.sessions.Revoke(ctx, sessionID)
		return TokenPair{}, err
	}
	if err := s.users.RecordLogin(ctx, account.ID); err != nil {
		_ = s.sessions.Revoke(ctx, sessionID)
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresAt: expiresAt, SessionID: sessionID, User: account}, nil
}

func (s *Service) Refresh(ctx context.Context, refresh string) (TokenPair, error) {
	newToken, newDigest, err := newRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}
	now := s.clock.Now().UTC()
	session, err := s.sessions.Rotate(ctx, digestToken(refresh), newDigest, now, s.refreshTTL)
	if err != nil {
		if !errors.Is(err, ErrInvalidToken) && !errors.Is(err, ErrRefreshReuse) {
			return TokenPair{}, fmt.Errorf("%w: 轮换认证会话: %w", ErrUnavailable, err)
		}
		return TokenPair{}, err
	}
	account, err := s.users.GetByID(ctx, session.UserID)
	if err != nil || account.Status != user.StatusActive || account.SessionVersion != session.UserVersion {
		_ = s.sessions.Revoke(ctx, session.ID)
		return TokenPair{}, ErrInvalidToken
	}
	access, expiresAt, err := s.tokens.Sign(account.ID, session.ID, account.SessionVersion, now)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: newToken, ExpiresAt: expiresAt, SessionID: session.ID, User: account}, nil
}

func (s *Service) Authenticate(ctx context.Context, access string) (Identity, error) {
	claims, err := s.tokens.Verify(access, s.clock.Now().UTC())
	if err != nil {
		return Identity{}, ErrInvalidToken
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return Identity{}, ErrInvalidToken
	}
	session, err := s.sessions.Get(ctx, claims.SessionID)
	if err != nil {
		if errors.Is(err, ErrInvalidToken) {
			return Identity{}, ErrInvalidToken
		}
		return Identity{}, fmt.Errorf("%w: 读取认证会话: %w", ErrUnavailable, err)
	}
	if session.UserID != userID || session.UserVersion != claims.Version {
		return Identity{}, ErrInvalidToken
	}
	account, err := s.users.GetByID(ctx, userID)
	if err != nil || account.Status != user.StatusActive || account.SessionVersion != claims.Version {
		return Identity{}, ErrInvalidToken
	}
	return Identity{User: account, SessionID: session.ID}, nil
}

func (s *Service) Logout(ctx context.Context, sessionID uuid.UUID) error {
	if err := s.sessions.Revoke(ctx, sessionID); err != nil {
		return fmt.Errorf("%w: 撤销认证会话: %w", ErrUnavailable, err)
	}
	return nil
}

func (s *Service) ListSessions(ctx context.Context, userID, currentSessionID uuid.UUID) ([]SessionSummary, error) {
	sessions, err := s.sessions.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("%w: 读取用户认证会话: %w", ErrUnavailable, err)
	}
	result := make([]SessionSummary, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, SessionSummary{
			ID:        session.ID,
			CreatedAt: session.CreatedAt,
			ExpiresAt: session.ExpiresAt,
			Current:   session.ID == currentSessionID,
		})
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].CreatedAt.Equal(result[right].CreatedAt) {
			return result[left].ID.String() < result[right].ID.String()
		}
		return result[left].CreatedAt.After(result[right].CreatedAt)
	})
	return result, nil
}

func (s *Service) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	session, err := s.sessions.Get(ctx, sessionID)
	if errors.Is(err, ErrInvalidToken) {
		return ErrSessionNotFound
	}
	if err != nil {
		return fmt.Errorf("%w: 读取认证会话: %w", ErrUnavailable, err)
	}
	if session.UserID != userID {
		return ErrSessionNotFound
	}
	if err := s.sessions.Revoke(ctx, sessionID); err != nil {
		return fmt.Errorf("%w: 撤销认证会话: %w", ErrUnavailable, err)
	}
	return nil
}

func (s *Service) RevokeOtherSessions(ctx context.Context, userID, currentSessionID uuid.UUID) error {
	sessions, err := s.sessions.ListByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("%w: 读取用户认证会话: %w", ErrUnavailable, err)
	}
	for _, session := range sessions {
		if session.ID == currentSessionID {
			continue
		}
		if err := s.sessions.Revoke(ctx, session.ID); err != nil {
			return fmt.Errorf("%w: 撤销其他认证会话: %w", ErrUnavailable, err)
		}
	}
	return nil
}

func (s *Service) RevokeUser(ctx context.Context, userID uuid.UUID) error {
	if err := s.sessions.RevokeUser(ctx, userID); err != nil {
		return fmt.Errorf("%w: 撤销用户会话: %w", ErrUnavailable, err)
	}
	return nil
}

func newRefreshToken() (string, string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", "", fmt.Errorf("生成 Refresh Token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(value)
	return token, digestToken(token), nil
}

func digestToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}
