// Package redisstore 提供 Redis 客户端生命周期与健康检查。
package redisstore

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/redis/go-redis/v9"
)

type Component struct {
	client        *redis.Client
	healthTimeout time.Duration
}

func New(cfg config.RedisConfig) *Component {
	options := &redis.Options{
		Addr: net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)), DB: cfg.Database,
		Username: cfg.Username, Password: cfg.Password.Reveal(), PoolSize: cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns, DialTimeout: cfg.ConnectTimeout,
		ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout,
	}
	if cfg.TLS {
		options.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: cfg.Host}
	}
	return &Component{client: redis.NewClient(options), healthTimeout: cfg.HealthTimeout}
}

func (c *Component) Start(ctx context.Context) error {
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis 连接失败: %w", err)
	}
	return nil
}

func (c *Component) Stop(context.Context) error {
	if err := c.client.Close(); err != nil {
		return fmt.Errorf("关闭 Redis 失败: %w", err)
	}
	return nil
}

func (c *Component) Health(ctx context.Context) error {
	healthCtx, cancel := context.WithTimeout(ctx, c.healthTimeout)
	defer cancel()
	if err := c.client.Ping(healthCtx).Err(); err != nil {
		return fmt.Errorf("redis 就绪检查失败: %w", err)
	}
	return nil
}

// Client 只供 Composition Root 注入基础设施适配器，业务层不得持有。
func (c *Component) Client() redis.UniversalClient { return c.client }
