// Package redis 实现动态配置公开列表的 Redis Cache-Aside 适配器。
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/downdawn/goba-slim/internal/modules/systemconfig"
	redisclient "github.com/redis/go-redis/v9"
)

type Cache struct {
	client redisclient.UniversalClient
	key    string
	ttl    time.Duration
}

func New(client redisclient.UniversalClient, environment string, ttl time.Duration) (*Cache, error) {
	if client == nil || environment == "" || ttl <= 0 {
		return nil, fmt.Errorf("动态配置 Redis 缓存依赖无效")
	}
	return &Cache{client: client, key: "goba:" + environment + ":systemconfig:public", ttl: ttl}, nil
}

func (c *Cache) Get(ctx context.Context) ([]systemconfig.Config, bool, error) {
	value, err := c.client.Get(ctx, c.key).Bytes()
	if errors.Is(err, redisclient.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("读取公开动态配置缓存: %w", err)
	}
	var items []systemconfig.Config
	if err := json.Unmarshal(value, &items); err != nil {
		return nil, false, fmt.Errorf("解析公开动态配置缓存: %w", err)
	}
	return items, true, nil
}

func (c *Cache) Put(ctx context.Context, items []systemconfig.Config) error {
	value, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("编码公开动态配置缓存: %w", err)
	}
	if err := c.client.Set(ctx, c.key, value, c.ttl).Err(); err != nil {
		return fmt.Errorf("写入公开动态配置缓存: %w", err)
	}
	return nil
}

func (c *Cache) Delete(ctx context.Context) error {
	if err := c.client.Del(ctx, c.key).Err(); err != nil {
		return fmt.Errorf("删除公开动态配置缓存: %w", err)
	}
	return nil
}

var _ systemconfig.PublicCache = (*Cache)(nil)
