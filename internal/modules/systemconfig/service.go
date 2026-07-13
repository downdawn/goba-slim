package systemconfig

import (
	"context"
	"fmt"

	"github.com/downdawn/goba-slim/internal/shared/clock"
)

type Service struct {
	repository Repository
	uow        UnitOfWork
	cache      PublicCache
	events     EventPublisher
	clock      clock.Clock
}

func NewService(repository Repository, uow UnitOfWork, cache PublicCache, events EventPublisher, businessClock clock.Clock) (*Service, error) {
	if repository == nil || uow == nil || cache == nil || events == nil || businessClock == nil {
		return nil, fmt.Errorf("动态配置服务依赖不能为空")
	}
	return &Service{repository: repository, uow: uow, cache: cache, events: events, clock: businessClock}, nil
}

func (s *Service) Create(ctx context.Context, input Input) (Config, error) {
	normalized, err := ValidateAndNormalize(input)
	if err != nil {
		return Config{}, err
	}
	now := s.clock.Now().UTC()
	item := Config{Key: normalized.Key, Value: normalized.Value, ValueType: normalized.ValueType, IsPublic: normalized.IsPublic, Description: normalized.Description, CreatedAt: now, UpdatedAt: now}
	err = s.uow.WithinTransaction(ctx, func(repository Repository) error {
		var createErr error
		item, createErr = repository.Create(ctx, item)
		return createErr
	})
	if err != nil {
		return Config{}, err
	}
	return item, s.afterCommit(ctx, ConfigChanged{Key: item.Key, ChangedAt: now})
}

func (s *Service) Get(ctx context.Context, key string) (Config, error) {
	if !configKeyPattern.MatchString(key) {
		return Config{}, ErrInvalidInput
	}
	return s.repository.Get(ctx, key)
}

func (s *Service) List(ctx context.Context) ([]Config, error) {
	return s.repository.List(ctx)
}

func (s *Service) ListPublic(ctx context.Context) ([]Config, error) {
	if items, hit, err := s.cache.Get(ctx); err == nil && hit {
		return items, nil
	}
	items, err := s.repository.ListPublic(ctx)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Put(ctx, items)
	return items, nil
}

func (s *Service) Update(ctx context.Context, key string, input Input) (Config, error) {
	input.Key = key
	normalized, err := ValidateAndNormalize(input)
	if err != nil {
		return Config{}, err
	}
	now := s.clock.Now().UTC()
	item := Config{Key: normalized.Key, Value: normalized.Value, ValueType: normalized.ValueType, IsPublic: normalized.IsPublic, Description: normalized.Description, UpdatedAt: now}
	err = s.uow.WithinTransaction(ctx, func(repository Repository) error {
		var updateErr error
		item, updateErr = repository.Update(ctx, item)
		return updateErr
	})
	if err != nil {
		return Config{}, err
	}
	return item, s.afterCommit(ctx, ConfigChanged{Key: item.Key, ChangedAt: now})
}

func (s *Service) Delete(ctx context.Context, key string) error {
	if !configKeyPattern.MatchString(key) {
		return ErrInvalidInput
	}
	if sensitiveKey(key) {
		return ErrSensitiveKey
	}
	now := s.clock.Now().UTC()
	if err := s.uow.WithinTransaction(ctx, func(repository Repository) error { return repository.Delete(ctx, key) }); err != nil {
		return err
	}
	return s.afterCommit(ctx, ConfigChanged{Key: key, Deleted: true, ChangedAt: now})
}

func (s *Service) afterCommit(ctx context.Context, event ConfigChanged) error {
	err := s.cache.Delete(ctx)
	s.events.Publish(ctx, event)
	if err != nil {
		return fmt.Errorf("%w: 数据库已提交，但公共配置缓存失效失败: %w", ErrPostCommit, err)
	}
	return nil
}
