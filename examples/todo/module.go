// Package todo 演示不默认装配进生产的最小业务模块。
package todo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/downdawn/goba-slim/internal/module"
	"github.com/downdawn/goba-slim/internal/shared/id"
	"github.com/google/uuid"
)

var ErrNotFound = errors.New("todo not found")

type Todo struct {
	ID    uuid.UUID
	Title string
	Done  bool
}

type Repository interface {
	Create(context.Context, Todo) (Todo, error)
	List(context.Context) ([]Todo, error)
}

type Service struct {
	repository Repository
	ids        id.Generator
}

func NewService(repository Repository, ids id.Generator) (*Service, error) {
	if repository == nil || ids == nil {
		return nil, fmt.Errorf("todo 服务依赖不能为空")
	}
	return &Service{repository: repository, ids: ids}, nil
}

func (s *Service) Create(ctx context.Context, title string) (Todo, error) {
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 200 {
		return Todo{}, fmt.Errorf("todo 标题必须为 1 到 200 个字符")
	}
	identifier, err := s.ids.New()
	if err != nil {
		return Todo{}, fmt.Errorf("生成 todo 标识: %w", err)
	}
	return s.repository.Create(ctx, Todo{ID: identifier, Title: title})
}

func (s *Service) List(ctx context.Context) ([]Todo, error) { return s.repository.List(ctx) }

type MemoryRepository struct {
	mu    sync.RWMutex
	items []Todo
}

func (r *MemoryRepository) Create(_ context.Context, item Todo) (Todo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = append(r.items, item)
	return item, nil
}

func (r *MemoryRepository) List(context.Context) ([]Todo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Todo(nil), r.items...), nil
}

type Module struct{ Service *Service }

func (*Module) Manifest() module.Manifest {
	return module.Manifest{Name: "todo", Requires: []string{"database"}}
}

func (*Module) Register(*module.Registry) error { return nil }

var _ module.Module = (*Module)(nil)
