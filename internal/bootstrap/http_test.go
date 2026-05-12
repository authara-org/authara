package bootstrap

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/cache"
	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/ratelimiter"
)

func TestNewAuthLimiterUsesInMemoryLimiterForNoopCache(t *testing.T) {
	app := &App{
		Config: &config.Config{
			Cache: config.Cache{Provider: "noop"},
		},
		Logger: slog.Default(),
		Cache:  cache.NewNoop(),
	}

	limiter := newAuthLimiter(app)

	if _, ok := limiter.(*ratelimiter.InMemoryLimiter); !ok {
		t.Fatalf("expected *InMemoryLimiter, got %T", limiter)
	}
}

func TestNewAuthLimiterUsesCacheLimiterForRedisCache(t *testing.T) {
	app := &App{
		Config: &config.Config{
			Cache: config.Cache{Provider: "redis"},
		},
		Logger: slog.Default(),
		Cache:  fakeBootstrapCounterCache{},
	}

	limiter := newAuthLimiter(app)

	if _, ok := limiter.(*ratelimiter.CacheLimiter); !ok {
		t.Fatalf("expected *CacheLimiter, got %T", limiter)
	}
}

type fakeBootstrapCounterCache struct{}

func (fakeBootstrapCounterCache) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, cache.ErrMiss
}

func (fakeBootstrapCounterCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return nil
}

func (fakeBootstrapCounterCache) Delete(ctx context.Context, key string) error {
	return nil
}

func (fakeBootstrapCounterCache) Close() error {
	return nil
}

func (fakeBootstrapCounterCache) Increment(
	ctx context.Context,
	key string,
	ttl time.Duration,
) (int64, time.Duration, error) {
	return 1, ttl, nil
}
