package cache

import (
	"context"
	"time"
)

// Cache defines the caching interface
type Cache interface {
	Get(ctx context.Context, key string) (interface{}, bool)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	RedisURL     string        `env:"REDIS_URL"`
	DefaultTTL   time.Duration `env:"CACHE_DEFAULT_TTL" default:"1h"`
	MaxMemoryMB  int           `env:"CACHE_MAX_MEMORY_MB" default:"512"`
	EphemerisTTL time.Duration `env:"EPHEMERIS_CACHE_TTL" default:"24h"`
}
