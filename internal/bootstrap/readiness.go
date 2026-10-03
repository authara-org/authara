package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/authara-org/authara/internal/cache"
	"github.com/authara-org/authara/internal/observability"
	"github.com/authara-org/authara/internal/store/schema"
)

type readinessStore interface {
	Ping(context.Context) error
	CurrentSchemaVersion(context.Context) (int, error)
}

type readinessChecker struct {
	store   readinessStore
	redis   cache.Pinger
	metrics *observability.Service
}

func newReadinessChecker(app *App) (*readinessChecker, error) {
	if app == nil || app.Store == nil {
		return nil, errors.New("readiness store is required")
	}
	if app.Config == nil {
		return nil, errors.New("readiness config is required")
	}

	checker := &readinessChecker{store: app.Store, metrics: app.Observability}
	if app.Config.Cache.Provider == "redis" {
		redis, ok := app.Cache.(cache.Pinger)
		if !ok {
			return nil, errors.New("configured Redis cache does not support readiness checks")
		}
		checker.redis = redis
	}
	return checker, nil
}

func (c *readinessChecker) Check(ctx context.Context) error {
	started := time.Now()
	if err := c.store.Ping(ctx); err != nil {
		c.observe("postgres", "failed", time.Since(started))
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}
	c.observe("postgres", "succeeded", time.Since(started))

	started = time.Now()
	current, err := c.store.CurrentSchemaVersion(ctx)
	if err != nil {
		c.observe("schema", "failed", time.Since(started))
		return fmt.Errorf("read schema version: %w", err)
	}
	if current != schema.RequiredSchemaVersion {
		c.observe("schema", "failed", time.Since(started))
		return fmt.Errorf(
			"schema version mismatch: current=%d required=%d",
			current,
			schema.RequiredSchemaVersion,
		)
	}
	c.observe("schema", "succeeded", time.Since(started))

	if c.redis != nil {
		started = time.Now()
		if err := c.redis.Ping(ctx); err != nil {
			c.observe("redis", "failed", time.Since(started))
			return fmt.Errorf("ping Redis: %w", err)
		}
		c.observe("redis", "succeeded", time.Since(started))
	}
	return nil
}

func (c *readinessChecker) observe(dependency, result string, duration time.Duration) {
	if c.metrics != nil {
		c.metrics.ObserveReadinessCheckDuration(dependency, result, duration)
	}
}
