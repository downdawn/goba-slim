package user

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fixedClock struct{ value time.Time }

func (c fixedClock) Now() time.Time { return c.value }

type fixedIDs struct{ value uuid.UUID }

func (g fixedIDs) New() (uuid.UUID, error) { return g.value, nil }

type memoryRepository struct{ users map[uuid.UUID]User }

func newMemoryRepository() *memoryRepository { return &memoryRepository{users: map[uuid.UUID]User{}} }

func (r *memoryRepository) Create(_ context.Context, item User) (User, error) {
	r.users[item.ID] = item
	return item, nil
}
func (r *memoryRepository) GetByID(_ context.Context, identifier uuid.UUID) (User, error) {
	item, ok := r.users[identifier]
	if !ok {
		return User{}, ErrNotFound
	}
	return item, nil
}
func (r *memoryRepository) GetByUsername(_ context.Context, username string) (User, error) {
	for _, item := range r.users {
		if item.Username == username {
			return item, nil
		}
	}
	return User{}, ErrNotFound
}
func (r *memoryRepository) List(context.Context, ListFilter, int32, int32) ([]User, int64, error) {
	items := make([]User, 0, len(r.users))
	for _, item := range r.users {
		items = append(items, item)
	}
	return items, int64(len(items)), nil
}
func (r *memoryRepository) UpdateProfile(ctx context.Context, identifier uuid.UUID, input UpdateProfileInput, now time.Time) (User, error) {
	item, err := r.GetByID(ctx, identifier)
	if err != nil {
		return User{}, err
	}
	item.Username, item.DisplayName = input.Username, input.DisplayName
	item.Email, item.AvatarURL, item.UpdatedAt = optionalString(input.Email), optionalString(input.AvatarURL), now
	r.users[identifier] = item
	return item, nil
}
func (r *memoryRepository) SetStatus(ctx context.Context, identifier uuid.UUID, status Status, now time.Time) (User, error) {
	item, err := r.GetByID(ctx, identifier)
	if err != nil {
		return User{}, err
	}
	item.Status, item.UpdatedAt = status, now
	r.users[identifier] = item
	return item, nil
}
func (r *memoryRepository) SetSuperuser(ctx context.Context, identifier uuid.UUID, enabled bool, now time.Time) (User, error) {
	item, err := r.GetByID(ctx, identifier)
	if err != nil {
		return User{}, err
	}
	item.IsSuperuser, item.UpdatedAt = enabled, now
	r.users[identifier] = item
	return item, nil
}
func (r *memoryRepository) SetMultipleSessions(ctx context.Context, identifier uuid.UUID, enabled bool, now time.Time) (User, error) {
	item, err := r.GetByID(ctx, identifier)
	if err != nil {
		return User{}, err
	}
	item.AllowMultipleSessions, item.UpdatedAt = enabled, now
	r.users[identifier] = item
	return item, nil
}
func (r *memoryRepository) UpdatePassword(ctx context.Context, identifier uuid.UUID, hash string, now time.Time) (User, error) {
	item, err := r.GetByID(ctx, identifier)
	if err != nil {
		return User{}, err
	}
	item.PasswordHash, item.PasswordChangedAt, item.UpdatedAt = hash, now, now
	r.users[identifier] = item
	return item, nil
}
func (r *memoryRepository) UpdatePasswordHash(ctx context.Context, identifier uuid.UUID, hash string, now time.Time) error {
	item, err := r.GetByID(ctx, identifier)
	if err != nil {
		return err
	}
	item.PasswordHash, item.UpdatedAt = hash, now
	r.users[identifier] = item
	return nil
}
func (*memoryRepository) UpdateLastLogin(context.Context, uuid.UUID, time.Time) error { return nil }
func (*memoryRepository) LockSuperuserChanges(context.Context) error                  { return nil }
func (r *memoryRepository) CountActiveSuperusers(context.Context) (int64, error) {
	var count int64
	for _, item := range r.users {
		if item.IsSuperuser && item.Status == StatusActive {
			count++
		}
	}
	return count, nil
}
func (r *memoryRepository) WithinTransaction(ctx context.Context, fn func(Repository) error) error {
	return fn(r)
}

func TestServiceCreatesNormalizedUser(t *testing.T) {
	repository := newMemoryRepository()
	passwords, err := NewPasswords(testArgon2Params())
	require.NoError(t, err)
	identifier := uuid.MustParse("019befd7-98d0-7000-8000-000000000001")
	now := time.Date(2026, time.July, 12, 12, 0, 0, 0, time.UTC)
	service, err := NewService(repository, repository, passwords, fixedClock{value: now}, fixedIDs{value: identifier})
	require.NoError(t, err)

	created, err := service.Create(t.Context(), CreateInput{Username: " Admin.User ", Password: "CorrectHorse9", Email: "ADMIN@example.com"})

	require.NoError(t, err)
	require.Equal(t, "admin.user", created.Username)
	require.Equal(t, "admin.user", created.DisplayName)
	require.Equal(t, "admin@example.com", *created.Email)
	require.Equal(t, identifier, created.ID)
	matched, err := passwords.Verify("CorrectHorse9", created.PasswordHash)
	require.NoError(t, err)
	require.True(t, matched)
}

func TestServiceProtectsLastActiveSuperuser(t *testing.T) {
	repository := newMemoryRepository()
	identifier := uuid.MustParse("019befd7-98d0-7000-8000-000000000002")
	repository.users[identifier] = User{ID: identifier, Username: "admin", IsSuperuser: true, Status: StatusActive}
	passwords, err := NewPasswords(testArgon2Params())
	require.NoError(t, err)
	service, err := NewService(repository, repository, passwords, fixedClock{value: time.Now()}, fixedIDs{value: identifier})
	require.NoError(t, err)

	_, err = service.SetStatus(t.Context(), identifier, StatusDisabled)
	require.ErrorIs(t, err, ErrLastSuperuser)
	_, err = service.SetSuperuser(t.Context(), identifier, false)
	require.ErrorIs(t, err, ErrLastSuperuser)
}

func TestVerifyCredentialsRehashesOldParametersWithoutRevokingSessions(t *testing.T) {
	repository := newMemoryRepository()
	oldPasswords, err := NewPasswords(testArgon2Params())
	require.NoError(t, err)
	currentParams := testArgon2Params()
	currentParams.Time = 2
	passwords, err := NewPasswords(currentParams)
	require.NoError(t, err)
	oldHash, err := oldPasswords.Hash("CorrectHorse9")
	require.NoError(t, err)
	identifier := uuid.MustParse("019befd7-98d0-7000-8000-000000000003")
	passwordChangedAt := time.Date(2026, time.July, 12, 12, 0, 0, 0, time.UTC)
	now := passwordChangedAt.Add(time.Hour)
	repository.users[identifier] = User{ID: identifier, Username: "admin", PasswordHash: oldHash, Status: StatusActive, SessionVersion: 7, PasswordChangedAt: passwordChangedAt}
	service, err := NewService(repository, repository, passwords, fixedClock{value: now}, fixedIDs{value: identifier})
	require.NoError(t, err)

	verified, err := service.VerifyCredentials(t.Context(), "admin", "CorrectHorse9")

	require.NoError(t, err)
	require.NotEqual(t, oldHash, verified.PasswordHash)
	require.False(t, passwords.NeedsRehash(verified.PasswordHash))
	require.Equal(t, int64(7), verified.SessionVersion)
	require.Equal(t, passwordChangedAt, verified.PasswordChangedAt)
	require.Equal(t, now, verified.UpdatedAt)
}
