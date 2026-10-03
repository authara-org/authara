package bootstrap

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
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

func TestTrustedProxyCIDRsWarnsAndDisablesTrustWithoutValidNetwork(t *testing.T) {
	var output bytes.Buffer
	app := &App{
		Config: newTestConfigService(t, &config.Config{Values: config.Values{
			TrustProxyHeaders:    true,
			TrustedProxyCIDRsRaw: "not-a-cidr",
		}}),
		Logger: slog.New(slog.NewTextHandler(&output, nil)),
	}

	if got := trustedProxyCIDRs(app); len(got) != 0 {
		t.Fatalf("trusted proxy CIDRs = %v, want none", got)
	}
	logs := output.String()
	for _, want := range []string{"invalid trusted proxy CIDR", "forwarded headers will be ignored"} {
		if !strings.Contains(logs, want) {
			t.Fatalf("logs = %q, want warning containing %q", logs, want)
		}
	}
}

func TestTrustedProxyCIDRsKeepsValidNetworksAndWarnsForTrustAll(t *testing.T) {
	var output bytes.Buffer
	app := &App{
		Config: newTestConfigService(t, &config.Config{Values: config.Values{
			TrustProxyHeaders:    true,
			TrustedProxyCIDRsRaw: "10.0.0.5/8, 0.0.0.0/0",
		}}),
		Logger: slog.New(slog.NewTextHandler(&output, nil)),
	}

	got := trustedProxyCIDRs(app)
	if len(got) != 2 || got[0].String() != "10.0.0.0/8" || got[1].String() != "0.0.0.0/0" {
		t.Fatalf("trusted proxy CIDRs = %v", got)
	}
	if !strings.Contains(output.String(), "permits every address") {
		t.Fatalf("logs = %q, want trust-all warning", output.String())
	}
}

func TestTrustedProxyCIDRsWarnsWhenNetworksAreConfiguredButTrustIsDisabled(t *testing.T) {
	var output bytes.Buffer
	app := &App{
		Config: newTestConfigService(t, &config.Config{Values: config.Values{
			TrustedProxyCIDRsRaw: "10.0.0.0/8",
		}}),
		Logger: slog.New(slog.NewTextHandler(&output, nil)),
	}

	if got := trustedProxyCIDRs(app); len(got) != 0 {
		t.Fatalf("trusted proxy CIDRs = %v, want none", got)
	}
	if !strings.Contains(output.String(), "proxy header trust is disabled") {
		t.Fatalf("logs = %q, want disabled-trust warning", output.String())
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

func (fakeBootstrapCounterCache) SetMaxInt64(ctx context.Context, key string, value int64, ttl time.Duration) error {
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
