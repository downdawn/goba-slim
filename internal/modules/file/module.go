package file

import (
	"context"

	"github.com/downdawn/goba-slim/internal/module"
)

type Module struct{ store *LocalStore }

func NewModule(store *LocalStore) *Module { return &Module{store: store} }

func (*Module) Manifest() module.Manifest {
	return module.Manifest{Name: "file", Requires: []string{"auth"}}
}

func (*Module) Register(*module.Registry) error { return nil }

func (m *Module) Start(ctx context.Context) error { return m.store.Start(ctx) }

func (m *Module) Stop(ctx context.Context) error { return m.store.Stop(ctx) }

func (m *Module) Health(ctx context.Context) error { return m.store.Health(ctx) }
