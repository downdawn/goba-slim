package app

import (
	"context"

	"github.com/downdawn/goba-slim/internal/module"
	"github.com/downdawn/goba-slim/internal/modules/auth"
	authredis "github.com/downdawn/goba-slim/internal/modules/auth/redis"
	"github.com/downdawn/goba-slim/internal/modules/user"
	userpostgres "github.com/downdawn/goba-slim/internal/modules/user/postgres"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/platform/database"
	"github.com/downdawn/goba-slim/internal/platform/redisstore"
	"github.com/downdawn/goba-slim/internal/shared/clock"
	"github.com/downdawn/goba-slim/internal/shared/id"
)

type coreComponents struct {
	database *database.Component
	redis    *redisstore.Component
	users    *user.Service
	auth     *auth.Service
}

func newUserComponents(cfg config.Config) (*coreComponents, error) {
	databaseComponent, err := database.New(cfg.Database)
	if err != nil {
		return nil, err
	}
	store, err := userpostgres.New(databaseComponent)
	if err != nil {
		return nil, err
	}
	passwords, err := user.NewPasswords(user.DefaultArgon2Params())
	if err != nil {
		return nil, err
	}
	userService, err := user.NewService(store, store, passwords, clock.System{}, id.UUIDv7{})
	if err != nil {
		return nil, err
	}
	return &coreComponents{database: databaseComponent, users: userService}, nil
}

func newCoreComponents(cfg config.Config) (*coreComponents, error) {
	components, err := newUserComponents(cfg)
	if err != nil {
		return nil, err
	}
	redisComponent := redisstore.New(cfg.Redis)
	store, err := authredis.New(redisComponent.Client(), cfg.App.Environment)
	if err != nil {
		return nil, err
	}
	tokens, err := auth.NewTokens(cfg.Auth)
	if err != nil {
		return nil, err
	}
	authService, err := auth.NewService(components.users, store, store, tokens, clock.System{}, cfg.Auth.RefreshTokenTTL, cfg.Auth.LoginAttempts, cfg.Auth.LoginWindow)
	if err != nil {
		return nil, err
	}
	components.redis = redisComponent
	components.auth = authService
	return components, nil
}

func buildCoreModules(cfg config.Config) ([]module.Module, *coreComponents, error) {
	components, err := newCoreComponents(cfg)
	if err != nil {
		return nil, nil, err
	}
	return []module.Module{
		&databaseModule{component: components.database},
		user.NewModule(components.users),
		&redisModule{component: components.redis},
		auth.NewModule(components.auth),
	}, components, nil
}

type databaseModule struct{ component *database.Component }

func (*databaseModule) Manifest() module.Manifest {
	return module.Manifest{Name: "database", Core: true}
}

func (*databaseModule) Register(*module.Registry) error { return nil }

func (m *databaseModule) Start(ctx context.Context) error { return m.component.Start(ctx) }

func (m *databaseModule) Stop(ctx context.Context) error { return m.component.Stop(ctx) }

func (m *databaseModule) Health(ctx context.Context) error { return m.component.Health(ctx) }

type redisModule struct{ component *redisstore.Component }

func (*redisModule) Manifest() module.Manifest {
	return module.Manifest{Name: "redis", Core: true}
}

func (*redisModule) Register(*module.Registry) error { return nil }

func (m *redisModule) Start(ctx context.Context) error { return m.component.Start(ctx) }

func (m *redisModule) Stop(ctx context.Context) error { return m.component.Stop(ctx) }

func (m *redisModule) Health(ctx context.Context) error { return m.component.Health(ctx) }
