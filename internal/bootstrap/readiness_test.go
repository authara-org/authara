package bootstrap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/store/schema"
)

func TestReadinessCheckerChecksRequiredDependencies(t *testing.T) {
	tests := []struct {
		name          string
		store         *fakeReadinessStore
		redis         *fakeReadinessPinger
		wantError     string
		wantSchemaHit bool
		wantRedisHit  bool
	}{
		{
			name:          "all dependencies ready",
			store:         &fakeReadinessStore{schemaVersion: schema.RequiredSchemaVersion},
			redis:         &fakeReadinessPinger{},
			wantSchemaHit: true,
			wantRedisHit:  true,
		},
		{
			name:          "Redis not configured",
			store:         &fakeReadinessStore{schemaVersion: schema.RequiredSchemaVersion},
			wantSchemaHit: true,
		},
		{
			name:      "PostgreSQL unavailable",
			store:     &fakeReadinessStore{pingErr: errors.New("unavailable")},
			redis:     &fakeReadinessPinger{},
			wantError: "ping PostgreSQL",
		},
		{
			name:          "schema query fails",
			store:         &fakeReadinessStore{schemaErr: errors.New("query failed")},
			redis:         &fakeReadinessPinger{},
			wantError:     "read schema version",
			wantSchemaHit: true,
		},
		{
			name:          "schema version mismatches",
			store:         &fakeReadinessStore{schemaVersion: schema.RequiredSchemaVersion - 1},
			redis:         &fakeReadinessPinger{},
			wantError:     "schema version mismatch",
			wantSchemaHit: true,
		},
		{
			name:          "Redis unavailable",
			store:         &fakeReadinessStore{schemaVersion: schema.RequiredSchemaVersion},
			redis:         &fakeReadinessPinger{err: errors.New("unavailable")},
			wantError:     "ping Redis",
			wantSchemaHit: true,
			wantRedisHit:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checker := &readinessChecker{store: test.store}
			if test.redis != nil {
				checker.redis = test.redis
			}
			err := checker.Check(context.Background())
			if test.wantError == "" && err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if test.wantError != "" && (err == nil || !strings.Contains(err.Error(), test.wantError)) {
				t.Fatalf("Check() error = %v, want containing %q", err, test.wantError)
			}
			if got := test.store.schemaCalls > 0; got != test.wantSchemaHit {
				t.Fatalf("schema checked = %t, want %t", got, test.wantSchemaHit)
			}
			redisHit := test.redis != nil && test.redis.calls > 0
			if redisHit != test.wantRedisHit {
				t.Fatalf("Redis checked = %t, want %t", redisHit, test.wantRedisHit)
			}
		})
	}
}

func TestReadinessCheckerDoesNotRequireRedisForNoopCache(t *testing.T) {
	app := &App{
		Config: newTestConfigService(t, &config.Config{Cache: config.Cache{Provider: "noop"}}),
		Store:  &store.Store{},
	}

	checker, err := newReadinessChecker(app)
	if err != nil {
		t.Fatal(err)
	}
	if checker.redis != nil {
		t.Fatal("noop cache unexpectedly configured as a readiness dependency")
	}
}

func TestReadinessCheckerRequiresConfiguredRedisPinger(t *testing.T) {
	app := &App{
		Config: newTestConfigService(t, &config.Config{Cache: config.Cache{Provider: "redis"}}),
		Store:  &store.Store{},
		Cache:  fakeBootstrapCounterCache{},
	}

	if _, err := newReadinessChecker(app); err == nil {
		t.Fatal("configured Redis without Ping support unexpectedly succeeded")
	}

	app.Cache = fakeBootstrapPingingCache{fakeBootstrapCounterCache: fakeBootstrapCounterCache{}}
	checker, err := newReadinessChecker(app)
	if err != nil {
		t.Fatal(err)
	}
	if checker.redis == nil {
		t.Fatal("configured Redis was not added as a readiness dependency")
	}
}

type fakeReadinessStore struct {
	pingErr       error
	schemaVersion int
	schemaErr     error
	schemaCalls   int
}

func (s *fakeReadinessStore) Ping(context.Context) error {
	return s.pingErr
}

func (s *fakeReadinessStore) CurrentSchemaVersion(context.Context) (int, error) {
	s.schemaCalls++
	return s.schemaVersion, s.schemaErr
}

type fakeReadinessPinger struct {
	err   error
	calls int
}

func (p *fakeReadinessPinger) Ping(context.Context) error {
	p.calls++
	return p.err
}

type fakeBootstrapPingingCache struct {
	fakeBootstrapCounterCache
}

func (fakeBootstrapPingingCache) Ping(context.Context) error {
	return nil
}
