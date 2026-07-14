package app

import (
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
