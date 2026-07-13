package systemconfig

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreateInvalidatesCacheAndPublishesOnlyAfterCommit(t *testing.T) {
	repository := &fakeRepository{}
	uow := &fakeUnitOfWork{repository: repository}
	cache := &fakeCache{}
	events := &fakeEvents{}
	service := newTestService(t, repository, uow, cache, events)

	created, err := service.Create(t.Context(), Input{Key: "feature.banner", Value: json.RawMessage(`true`), ValueType: TypeBoolean, IsPublic: true})
	require.NoError(t, err)
	require.Equal(t, "feature.banner", created.Key)
	require.True(t, uow.committed)
	require.Equal(t, []string{"transaction", "commit", "cache_delete", "event"}, append(append([]string{}, uow.order...), append(cache.order, events.order...)...))
	require.Equal(t, "feature.banner", events.last.Key)
}

func TestCreateDoesNotInvalidateCacheWhenTransactionFails(t *testing.T) {
	cause := errors.New("commit failed")
	repository := &fakeRepository{}
	uow := &fakeUnitOfWork{repository: repository, err: cause}
	cache := &fakeCache{}
	events := &fakeEvents{}
	service := newTestService(t, repository, uow, cache, events)

	_, err := service.Create(t.Context(), Input{Key: "feature.banner", Value: json.RawMessage(`true`), ValueType: TypeBoolean})
	require.ErrorIs(t, err, cause)
	require.Empty(t, cache.order)
	require.Empty(t, events.order)
}

func TestListPublicUsesCacheAndFallsBackWhenCacheFails(t *testing.T) {
	databaseItems := []Config{{Key: "feature.db", Value: json.RawMessage(`true`), ValueType: TypeBoolean, IsPublic: true}}
	repository := &fakeRepository{public: databaseItems}
	uow := &fakeUnitOfWork{repository: repository}
	cache := &fakeCache{items: []Config{{Key: "feature.cache", Value: json.RawMessage(`false`), ValueType: TypeBoolean, IsPublic: true}}, hit: true}
	service := newTestService(t, repository, uow, cache, &fakeEvents{})

	items, err := service.ListPublic(t.Context())
	require.NoError(t, err)
	require.Equal(t, "feature.cache", items[0].Key)
	require.Equal(t, 0, repository.publicReads)

	cache.hit = false
	cache.getErr = errors.New("redis down")
	items, err = service.ListPublic(t.Context())
	require.NoError(t, err)
	require.Equal(t, "feature.db", items[0].Key)
	require.Equal(t, 1, repository.publicReads)
}

func newTestService(t *testing.T, repository Repository, uow UnitOfWork, cache PublicCache, events EventPublisher) *Service {
	t.Helper()
	service, err := NewService(repository, uow, cache, events, fixedClock{now: time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)})
	require.NoError(t, err)
	return service
}

type fakeRepository struct {
	items       []Config
	public      []Config
	publicReads int
}

func (f *fakeRepository) Create(_ context.Context, item Config) (Config, error) {
	f.items = append(f.items, item)
	return item, nil
}
func (f *fakeRepository) Get(context.Context, string) (Config, error) { return Config{}, ErrNotFound }
func (f *fakeRepository) List(context.Context) ([]Config, error) {
	return append([]Config(nil), f.items...), nil
}
func (f *fakeRepository) ListPublic(context.Context) ([]Config, error) {
	f.publicReads++
	return append([]Config(nil), f.public...), nil
}
func (f *fakeRepository) Update(_ context.Context, item Config) (Config, error) { return item, nil }
func (f *fakeRepository) Delete(context.Context, string) error                  { return nil }

type fakeUnitOfWork struct {
	repository Repository
	err        error
	committed  bool
	order      []string
}

func (f *fakeUnitOfWork) WithinTransaction(ctx context.Context, fn func(Repository) error) error {
	f.order = append(f.order, "transaction")
	if err := fn(f.repository); err != nil {
		return err
	}
	if f.err != nil {
		return f.err
	}
	f.committed = true
	f.order = append(f.order, "commit")
	return nil
}

type fakeCache struct {
	items  []Config
	hit    bool
	getErr error
	order  []string
}

func (f *fakeCache) Get(context.Context) ([]Config, bool, error) { return f.items, f.hit, f.getErr }
func (f *fakeCache) Put(_ context.Context, items []Config) error {
	f.items = append([]Config(nil), items...)
	return nil
}
func (f *fakeCache) Delete(context.Context) error {
	f.order = append(f.order, "cache_delete")
	return nil
}

type fakeEvents struct {
	last  ConfigChanged
	order []string
}

func (f *fakeEvents) Publish(_ context.Context, event ConfigChanged) {
	f.last = event
	f.order = append(f.order, "event")
}

type fixedClock struct{ now time.Time }

func (f fixedClock) Now() time.Time { return f.now }
