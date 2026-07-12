// Package module 提供模块声明、依赖解析和生命周期管理。
package module

import (
	"context"
	"fmt"
	"sort"
)

type Manifest struct {
	Name     string
	Requires []string
	Core     bool
}
type Module interface {
	Manifest() Manifest
	Register(*Registry) error
}
type Starter interface{ Start(context.Context) error }
type Stopper interface{ Stop(context.Context) error }
type HealthChecker interface{ Health(context.Context) error }
type Registry struct{ modules map[string]Module }

func NewRegistry() *Registry { return &Registry{modules: make(map[string]Module)} }
func (r *Registry) Add(item Module) error {
	if item == nil || item.Manifest().Name == "" {
		return fmt.Errorf("模块名称不能为空")
	}
	name := item.Manifest().Name
	if _, exists := r.modules[name]; exists {
		return fmt.Errorf("模块 %q 已注册", name)
	}
	r.modules[name] = item
	return nil
}
func (r *Registry) Resolve(_ []string) ([]Module, error) {
	names := make([]string, 0, len(r.modules))
	for name := range r.modules {
		names = append(names, name)
	}
	sort.Strings(names)
	state := map[string]uint8{}
	ordered := make([]Module, 0, len(names))
	var visit func(string) error
	visit = func(name string) error {
		switch state[name] {
		case 1:
			return fmt.Errorf("模块依赖存在循环: %s", name)
		case 2:
			return nil
		}
		item, ok := r.modules[name]
		if !ok {
			return fmt.Errorf("未注册依赖模块 %q", name)
		}
		state[name] = 1
		requires := append([]string(nil), item.Manifest().Requires...)
		sort.Strings(requires)
		for _, dep := range requires {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[name] = 2
		ordered = append(ordered, item)
		return nil
	}
	for _, name := range names {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

type Runtime struct {
	modules []Module
	started []Module
}

func NewRuntime(modules []Module) *Runtime {
	return &Runtime{modules: append([]Module(nil), modules...)}
}
func (r *Runtime) Start(ctx context.Context) error {
	for _, item := range r.modules {
		if starter, ok := item.(Starter); ok {
			if err := starter.Start(ctx); err != nil {
				_ = r.Stop(ctx)
				return err
			}
		}
		r.started = append(r.started, item)
	}
	return nil
}
func (r *Runtime) Stop(ctx context.Context) error {
	var first error
	for index := len(r.started) - 1; index >= 0; index-- {
		if stopper, ok := r.started[index].(Stopper); ok {
			if err := stopper.Stop(ctx); err != nil && first == nil {
				first = err
			}
		}
	}
	r.started = nil
	return first
}
