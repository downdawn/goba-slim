//go:build integration

package tests

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/downdawn/goba-slim/internal/modules/user"
	userpostgres "github.com/downdawn/goba-slim/internal/modules/user/postgres"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/platform/database"
	"github.com/downdawn/goba-slim/internal/shared/clock"
	"github.com/downdawn/goba-slim/internal/shared/id"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestPhase2PostgreSQLWorkflow(t *testing.T) {
	cfg := startPostgreSQL(t)

	status, err := database.Inspect(t.Context(), cfg.Database)
	require.NoError(t, err)
	require.False(t, status.Initialized)

	pool, err := database.Open(t.Context(), cfg.Database)
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), "CREATE TABLE unexpected_table (id bigint PRIMARY KEY)")
	require.NoError(t, err)
	pool.Close()
	_, err = database.Migrate(t.Context(), cfg.Database)
	require.ErrorIs(t, err, database.ErrDatabaseNotEmpty)

	pool, err = database.Open(t.Context(), cfg.Database)
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), "DROP TABLE unexpected_table")
	require.NoError(t, err)
	pool.Close()

	result, err := database.Migrate(t.Context(), cfg.Database)
	require.NoError(t, err)
	require.Equal(t, int32(1), result.Applied)
	status, err = database.Inspect(t.Context(), cfg.Database)
	require.NoError(t, err)
	require.True(t, status.Initialized)
	require.Equal(t, status.Expected, status.SchemaVersion)
	result, err = database.Migrate(t.Context(), cfg.Database)
	require.NoError(t, err)
	require.Zero(t, result.Applied)

	pool, err = database.Open(t.Context(), cfg.Database)
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), "UPDATE public.goba_schema_version SET version = 2")
	require.NoError(t, err)
	_, err = database.Inspect(t.Context(), cfg.Database)
	require.ErrorIs(t, err, database.ErrSchemaMismatch)
	_, err = pool.Exec(t.Context(), "UPDATE public.goba_schema_version SET version = 1")
	require.NoError(t, err)
	pool.Close()

	component, err := database.New(cfg.Database)
	require.NoError(t, err)
	require.NoError(t, component.Start(t.Context()))
	t.Cleanup(func() { require.NoError(t, component.Stop(context.Background())) })
	store, err := userpostgres.New(component)
	require.NoError(t, err)
	passwords, err := user.NewPasswords(user.Argon2Params{MemoryKiB: 8 * 1024, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32})
	require.NoError(t, err)
	service, err := user.NewService(store, store, passwords, clock.System{}, id.UUIDv7{})
	require.NoError(t, err)

	admin, err := service.CreateAdmin(t.Context(), user.CreateInput{Username: "admin", Password: "AdminPassword9"})
	require.NoError(t, err)
	require.True(t, admin.IsSuperuser)
	_, err = service.SetStatus(t.Context(), admin.ID, user.StatusDisabled)
	require.ErrorIs(t, err, user.ErrLastSuperuser)

	second, err := service.CreateAdmin(t.Context(), user.CreateInput{Username: "admin.two", Password: "AdminPassword9"})
	require.NoError(t, err)
	_, err = service.SetStatus(t.Context(), admin.ID, user.StatusDisabled)
	require.NoError(t, err)
	page, err := service.List(t.Context(), user.ListFilter{Page: 1, Size: 10})
	require.NoError(t, err)
	require.Equal(t, int64(2), page.Total)
	require.Len(t, page.Items, 2)

	testTransactionRollback(t, store, passwords)
	testConcurrentUsernameConstraint(t, service)
	_, err = service.GetByID(t.Context(), second.ID)
	require.NoError(t, err)
}

func TestDatabaseMigrationSerializesConcurrentCalls(t *testing.T) {
	cfg := startPostgreSQL(t)
	results := make(chan struct {
		result database.MigrationResult
		err    error
	}, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	for range 2 {
		go func() {
			defer wait.Done()
			result, err := database.Migrate(t.Context(), cfg.Database)
			results <- struct {
				result database.MigrationResult
				err    error
			}{result: result, err: err}
		}()
	}
	wait.Wait()
	close(results)

	var applied int32
	for item := range results {
		require.NoError(t, item.err)
		applied += item.result.Applied
	}
	require.Equal(t, int32(1), applied)
	status, err := database.Inspect(t.Context(), cfg.Database)
	require.NoError(t, err)
	require.True(t, status.Initialized)
}

func TestDatabaseMigrationFailureRollsBack(t *testing.T) {
	cfg := startPostgreSQL(t)
	pool, err := database.Open(t.Context(), cfg.Database)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	_, err = pool.Exec(t.Context(), "CREATE TABLE public.goba_schema_version (version int4 NOT NULL PRIMARY KEY)")
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), "INSERT INTO public.goba_schema_version (version) VALUES (0)")
	require.NoError(t, err)
	_, err = pool.Exec(t.Context(), "CREATE TABLE public.system_configs (id bigint PRIMARY KEY)")
	require.NoError(t, err)

	_, err = database.Migrate(t.Context(), cfg.Database)
	require.Error(t, err)

	var version int32
	require.NoError(t, pool.QueryRow(t.Context(), "SELECT version FROM public.goba_schema_version").Scan(&version))
	require.Zero(t, version)
	var usersTable *string
	require.NoError(t, pool.QueryRow(t.Context(), "SELECT to_regclass('public.users')").Scan(&usersTable))
	require.Nil(t, usersTable)
}

func testTransactionRollback(t *testing.T, store *userpostgres.Store, passwords *user.Passwords) {
	t.Helper()
	hash, err := passwords.Hash("RollbackPassword9")
	require.NoError(t, err)
	identifier, err := uuid.NewV7()
	require.NoError(t, err)
	now := time.Now().UTC()
	cause := errors.New("rollback requested")
	err = store.WithinTransaction(t.Context(), func(repository user.Repository) error {
		_, createErr := repository.Create(t.Context(), user.User{
			ID: identifier, Username: "rollback.user", PasswordHash: hash, DisplayName: "rollback.user",
			Status: user.StatusActive, SessionVersion: 1, PasswordChangedAt: now, CreatedAt: now, UpdatedAt: now,
		})
		if createErr != nil {
			return createErr
		}
		return cause
	})
	require.ErrorIs(t, err, cause)
	_, err = store.GetByID(t.Context(), identifier)
	require.ErrorIs(t, err, user.ErrNotFound)
}

func testConcurrentUsernameConstraint(t *testing.T, service *user.Service) {
	t.Helper()
	results := make(chan error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	for range 2 {
		go func() {
			defer wait.Done()
			_, err := service.Create(t.Context(), user.CreateInput{Username: "same.user", Password: "ConcurrentPassword9"})
			results <- err
		}()
	}
	wait.Wait()
	close(results)
	var successes, conflicts int
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, user.ErrUsernameConflict) {
			conflicts++
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
}

func startPostgreSQL(t *testing.T) config.Config {
	t.Helper()
	passwordBytes := make([]byte, 24)
	_, err := rand.Read(passwordBytes)
	require.NoError(t, err)
	password := "A1" + base64.RawURLEncoding.EncodeToString(passwordBytes)
	container, err := tcpostgres.Run(
		t.Context(),
		"postgres:17-alpine",
		tcpostgres.WithDatabase("goba"),
		tcpostgres.WithUsername("goba"),
		tcpostgres.WithPassword(password),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testcontainers.TerminateContainer(container)) })
	host, err := container.Host(t.Context())
	require.NoError(t, err)
	port, err := container.MappedPort(t.Context(), "5432/tcp")
	require.NoError(t, err)
	portNumber, err := strconv.Atoi(port.Port())
	require.NoError(t, err)
	cfg := config.Default()
	cfg.App.Environment = "test"
	cfg.Database.Host = host
	cfg.Database.Port = portNumber
	cfg.Database.Name = "goba"
	cfg.Database.User = "goba"
	cfg.Database.Password = config.NewSecret(password)
	cfg.Database.MinConnections = 0
	cfg.Database.MaxConnections = 5
	return cfg
}
