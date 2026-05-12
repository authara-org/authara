package cache

import (
	"context"
	"time"
)

type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Close() error
}

type Counter interface {
	Increment(ctx context.Context, key string, ttl time.Duration) (count int64, remainingTTL time.Duration, err error)
}
