//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/downdawn/goba-slim/internal/modules/systemconfig"
	systemconfigpostgres "github.com/downdawn/goba-slim/internal/modules/systemconfig/postgres"
	systemconfigredis "github.com/downdawn/goba-slim/internal/modules/systemconfig/redis"
	"github.com/downdawn/goba-slim/internal/platform/database"
	"github.com/downdawn/goba-slim/internal/shared/clock"
	redisclient "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestPhase5SystemConfigWorkflow(t *testing.T) {
	cfg := startPostgreSQL(t)
	_, err := database.Migrate(t.Context(), cfg.Database)
	require.NoError(t, err)
	databaseComponent, err := database.New(cfg.Database)
	require.NoError(t, err)
	require.NoError(t, databaseComponent.Start(t.Context()))
	t.Cleanup(func() { require.NoError(t, databaseComponent.Stop(context.Background())) })

	redisContainer, err := tcredis.Run(t.Context(), "redis:7-alpine")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testcontainers.TerminateContainer(redisContainer)) })
	connection, err := redisContainer.ConnectionString(t.Context())
	require.NoError(t, err)
	redisOptions, err := redisclient.ParseURL(connection)
	require.NoError(t, err)
	redisClient := redisclient.NewClient(redisOptions)
	t.Cleanup(func() { require.NoError(t, redisClient.Close()) })

	store, err := systemconfigpostgres.New(databaseComponent)
	require.NoError(t, err)
	cache, err := systemconfigredis.New(redisClient, "test", time.Minute)
	require.NoError(t, err)
	service, err := systemconfig.NewService(store, store, cache, systemconfig.NewBus(), clock.System{})
	require.NoError(t, err)

	created, err := service.Create(t.Context(), systemconfig.Input{
		Key: "feature.banner", Value: json.RawMessage(`true`), ValueType: systemconfig.TypeBoolean, IsPublic: true,
	})
	require.NoError(t, err)
	require.Equal(t, "feature.banner", created.Key)

	public, err := service.ListPublic(t.Context())
	require.NoError(t, err)
	require.Len(t, public, 1)
	_, hit, err := cache.Get(t.Context())
	require.NoError(t, err)
	require.True(t, hit)

	updated, err := service.Update(t.Context(), "feature.banner", systemconfig.Input{
		Value: json.RawMessage(`false`), ValueType: systemconfig.TypeBoolean, IsPublic: true,
	})
	require.NoError(t, err)
	require.JSONEq(t, `false`, string(updated.Value))
	_, hit, err = cache.Get(t.Context())
	require.NoError(t, err)
	require.False(t, hit)

	require.NoError(t, service.Delete(t.Context(), "feature.banner"))
	_, err = service.Get(t.Context(), "feature.banner")
	require.ErrorIs(t, err, systemconfig.ErrNotFound)
}
