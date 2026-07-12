package app

import (
	"context"

	"github.com/downdawn/goba-slim/internal/module"
	"github.com/downdawn/goba-slim/internal/modules/user"
	userpostgres "github.com/downdawn/goba-slim/internal/modules/user/postgres"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/platform/database"
	"github.com/downdawn/goba-slim/internal/shared/clock"
	"github.com/downdawn/goba-slim/internal/shared/id"
)

type coreComponents struct {
	database *database.Component
	users    *user.Service
}

func newCoreComponents(cfg config.Config) (*coreComponents, error) {
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

func buildCoreModules(cfg config.Config) ([]module.Module, error) {
	components, err := newCoreComponents(cfg)
	if err != nil {
		return nil, err
	}
	return []module.Module{
		&databaseModule{component: components.database},
		user.NewModule(components.users),
	}, nil
}

type databaseModule struct{ component *database.Component }

func (*databaseModule) Manifest() module.Manifest {
	return module.Manifest{Name: "database", Core: true}
}

func (*databaseModule) Register(*module.Registry) error { return nil }

func (m *databaseModule) Start(ctx context.Context) error { return m.component.Start(ctx) }

func (m *databaseModule) Stop(ctx context.Context) error { return m.component.Stop(ctx) }

func (m *databaseModule) Health(ctx context.Context) error { return m.component.Health(ctx) }
