package auth

import "github.com/downdawn/goba-slim/internal/module"

type Module struct{ Service *Service }

func NewModule(service *Service) *Module { return &Module{Service: service} }

func (*Module) Manifest() module.Manifest {
	return module.Manifest{Name: "auth", Requires: []string{"redis", "user"}, Core: true}
}

func (*Module) Register(*module.Registry) error { return nil }
