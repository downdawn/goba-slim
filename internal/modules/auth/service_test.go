package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/downdawn/goba-slim/internal/modules/user"
	"github.com/downdawn/goba-slim/internal/shared/clock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestLoginLimitsRotatingUnknownAccountsByIP(t *testing.T) {
	users := &fakeUsers{verifyErr: user.ErrNotFound}
	limiter := &fakeLimiter{counts: make(map[string]int)}
	service := newTestService(t, users, limiter)

	for _, username := range []string{"missing-one", "missing-two"} {
		_, err := service.Login(t.Context(), username, "password", "203.0.113.10")
		require.ErrorIs(t, err, ErrInvalidCredentials)
	}
	_, err := service.Login(t.Context(), "missing-three", "password", "203.0.113.10")

	require.ErrorIs(t, err, ErrRateLimited)
	require.Equal(t, 3, limiter.counts["ip:203.0.113.10"])
	require.Equal(t, 2, users.verifyCalls)
}

func TestLoginLimitsOneAccountAcrossDifferentIPs(t *testing.T) {
	users := &fakeUsers{verifyErr: user.ErrNotFound}
	limiter := &fakeLimiter{counts: make(map[string]int)}
	service := newTestService(t, users, limiter)

	_, err := service.Login(t.Context(), "missing-user", "password", "203.0.113.10")
	require.ErrorIs(t, err, ErrInvalidCredentials)
	_, err = service.Login(t.Context(), "missing-user", "password", "203.0.113.11")
	require.ErrorIs(t, err, ErrInvalidCredentials)
	_, err = service.Login(t.Context(), "missing-user", "password", "203.0.113.12")

	require.ErrorIs(t, err, ErrRateLimited)
	require.Equal(t, 3, limiter.counts["account:missing-user"])
}

func TestListSessionsSortsAndMarksCurrentSession(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())
	currentID := uuid.Must(uuid.NewV7())
	earlierID := uuid.Must(uuid.NewV7())
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	sessions := &fakeSessions{sessions: map[uuid.UUID]Session{
		currentID: {ID: currentID, UserID: userID, CreatedAt: now, ExpiresAt: now.Add(time.Hour)},
		earlierID: {ID: earlierID, UserID: userID, CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)},
	}}
	service := newTestServiceWithSessions(t, &fakeUsers{}, &fakeLimiter{counts: make(map[string]int)}, sessions)

	items, err := service.ListSessions(t.Context(), userID, currentID)

	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{currentID, earlierID}, []uuid.UUID{items[0].ID, items[1].ID})
	require.True(t, items[0].Current)
	require.False(t, items[1].Current)
}

func TestRevokeSessionDoesNotRevealOtherUsersSession(t *testing.T) {
	ownerID := uuid.Must(uuid.NewV7())
	otherID := uuid.Must(uuid.NewV7())
	sessionID := uuid.Must(uuid.NewV7())
	sessions := &fakeSessions{sessions: map[uuid.UUID]Session{
		sessionID: {ID: sessionID, UserID: otherID},
	}}
	service := newTestServiceWithSessions(t, &fakeUsers{}, &fakeLimiter{counts: make(map[string]int)}, sessions)

	err := service.RevokeSession(t.Context(), ownerID, sessionID)

	require.ErrorIs(t, err, ErrSessionNotFound)
	require.Empty(t, sessions.revoked)
}

func TestRevokeOtherSessionsKeepsCurrentSession(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())
	currentID := uuid.Must(uuid.NewV7())
	otherID := uuid.Must(uuid.NewV7())
	sessions := &fakeSessions{sessions: map[uuid.UUID]Session{
		currentID: {ID: currentID, UserID: userID},
		otherID:   {ID: otherID, UserID: userID},
	}}
	service := newTestServiceWithSessions(t, &fakeUsers{}, &fakeLimiter{counts: make(map[string]int)}, sessions)

	err := service.RevokeOtherSessions(t.Context(), userID, currentID)

	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{otherID}, sessions.revoked)
}

type fakeUsers struct {
	verifyCalls int
	verifyErr   error
}

func (u *fakeUsers) VerifyCredentials(context.Context, string, string) (user.User, error) {
	u.verifyCalls++
	return user.User{}, u.verifyErr
}

func (*fakeUsers) GetByID(context.Context, uuid.UUID) (user.User, error) {
	return user.User{}, user.ErrNotFound
}

func (*fakeUsers) RecordLogin(context.Context, uuid.UUID) error { return nil }

type fakeSessions struct {
	sessions map[uuid.UUID]Session
	revoked  []uuid.UUID
}

func (*fakeSessions) Create(context.Context, Session, time.Duration, bool) error { return nil }
func (s *fakeSessions) Get(_ context.Context, sessionID uuid.UUID) (Session, error) {
	session, ok := s.sessions[sessionID]
	if !ok {
		return Session{}, ErrInvalidToken
	}
	return session, nil
}
func (s *fakeSessions) ListByUser(_ context.Context, userID uuid.UUID) ([]Session, error) {
	items := make([]Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		if session.UserID == userID {
			items = append(items, session)
		}
	}
	return items, nil
}
func (*fakeSessions) Rotate(context.Context, string, string, time.Time, time.Duration) (Session, error) {
	return Session{}, ErrInvalidToken
}
func (s *fakeSessions) Revoke(_ context.Context, sessionID uuid.UUID) error {
	s.revoked = append(s.revoked, sessionID)
	delete(s.sessions, sessionID)
	return nil
}
func (*fakeSessions) RevokeUser(context.Context, uuid.UUID) error { return nil }

type fakeLimiter struct{ counts map[string]int }

func (l *fakeLimiter) Allow(_ context.Context, key string, limit int, _ time.Duration) (bool, error) {
	if l.counts == nil {
		return false, errors.New("missing counts")
	}
	l.counts[key]++
	return l.counts[key] <= limit, nil
}

func newTestService(t *testing.T, users UserService, limiter RateLimiter) *Service {
	return newTestServiceWithSessions(t, users, limiter, &fakeSessions{})
}

func newTestServiceWithSessions(t *testing.T, users UserService, limiter RateLimiter, sessions SessionStore) *Service {
	t.Helper()
	service, err := NewService(users, sessions, limiter, &Tokens{}, clock.System{}, time.Hour, 2, time.Minute)
	require.NoError(t, err)
	return service
}
