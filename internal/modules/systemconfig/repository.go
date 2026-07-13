package systemconfig

import "context"

type Repository interface {
	Create(context.Context, Config) (Config, error)
	Get(context.Context, string) (Config, error)
	List(context.Context) ([]Config, error)
	ListPublic(context.Context) ([]Config, error)
	Update(context.Context, Config) (Config, error)
	Delete(context.Context, string) error
}

type UnitOfWork interface {
	WithinTransaction(context.Context, func(Repository) error) error
}

type PublicCache interface {
	Get(context.Context) ([]Config, bool, error)
	Put(context.Context, []Config) error
	Delete(context.Context) error
}

type EventPublisher interface {
	Publish(context.Context, ConfigChanged)
}
