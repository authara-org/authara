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
		Config: newTestConfigService(t, &config.Config{
			Cache: config.Cache{Provider: "noop"},
		}),
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
		Config: newTestConfigService(t, &config.Config{
			Cache: config.Cache{Provider: "redis"},
		}),
		Logger: slog.Default(),
		Cache:  fakeBootstrapCounterCache{},
	}

	limiter := newAuthLimiter(app)

	if _, ok := limiter.(*ratelimiter.CacheLimiter); !ok {
		t.Fatalf("expected *CacheLimiter, got %T", limiter)
	}
}

func TestNewLimiterConfigReadsRuntimePolicy(t *testing.T) {
	app := &App{
		Config: newTestConfigService(t, &config.Config{RateLimit: config.RateLimit{LoginIPLimit: 999}}),
	}

	if got := newLimiterConfig(app).LoginIPLimit; got != 5 {
		t.Fatalf("limiter config login IP limit = %d, want runtime default 5", got)
	}
}

func newTestConfigService(t *testing.T, startup *config.Config) *config.Service {
	t.Helper()
	service, err := config.NewService(context.Background(), config.ServiceOptions{
		Startup:           startup,
		Store:             emptyRuntimeSettingsStore{},
		LookupEnvironment: func(string) (string, bool) { return "", false },
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

type emptyRuntimeSettingsStore struct {
	config.RuntimeSettingsStore
}

func (emptyRuntimeSettingsStore) LoadRuntimeSettings(context.Context) (config.PersistedState, error) {
	return config.PersistedState{}, nil
}

type fakeBootstrapCounterCache struct{}

func (fakeBootstrapCounterCache) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, cache.ErrMiss
}

func (fakeBootstrapCounterCache) GetMany(ctx context.Context, keys ...string) ([][]byte, error) {
	return make([][]byte, len(keys)), nil
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
