//go:build integration

package tests

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/downdawn/goba-slim/internal/modules/auth"
	authredis "github.com/downdawn/goba-slim/internal/modules/auth/redis"
	"github.com/google/uuid"
	redisclient "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestRedisRefreshRotationAndReuseRevocation(t *testing.T) {
	container, err := tcredis.Run(t.Context(), "redis:7-alpine")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testcontainers.TerminateContainer(container)) })
	connection, err := container.ConnectionString(t.Context())
	require.NoError(t, err)
	options, err := redisclient.ParseURL(connection)
	require.NoError(t, err)
	client := redisclient.NewClient(options)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	store, err := authredis.New(client, "test")
	require.NoError(t, err)

	now := time.Now().UTC()
	session := auth.Session{
		ID: uuid.Must(uuid.NewV7()), FamilyID: uuid.Must(uuid.NewV7()), UserID: uuid.Must(uuid.NewV7()),
		UserVersion: 1, CurrentDigest: "old-digest", CreatedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	require.NoError(t, store.Create(t.Context(), session, time.Hour, true))

	listed, err := store.ListByUser(t.Context(), session.UserID)
	require.NoError(t, err)
	require.Equal(t, []auth.Session{session}, listed)

	results := make(chan error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	for index := range 2 {
		go func() {
			defer wait.Done()
			_, rotateErr := store.Rotate(t.Context(), "old-digest", "new-digest-"+string(rune('a'+index)), now, time.Hour)
			results <- rotateErr
		}()
	}
	wait.Wait()
	close(results)
	var successes, reuses int
	for result := range results {
		if result == nil {
			successes++
		} else if errors.Is(result, auth.ErrRefreshReuse) {
			reuses++
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, reuses)
	_, err = store.Get(t.Context(), session.ID)
	require.ErrorIs(t, err, auth.ErrInvalidToken)

	allowed, err := store.Allow(t.Context(), "login", 1, time.Minute)
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, err = store.Allow(t.Context(), "login", 1, time.Minute)
	require.NoError(t, err)
	require.False(t, allowed)
}
