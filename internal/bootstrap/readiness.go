package bootstrap

import (
	"context"
	"errors"
	"fmt"

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
	if err := c.store.Ping(ctx); err != nil {
		c.observe("postgres", "failed")
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}
	c.observe("postgres", "succeeded")

	current, err := c.store.CurrentSchemaVersion(ctx)
	if err != nil {
		c.observe("schema", "failed")
		return fmt.Errorf("read schema version: %w", err)
	}
	if current != schema.RequiredSchemaVersion {
		c.observe("schema", "failed")
		return fmt.Errorf(
			"schema version mismatch: current=%d required=%d",
			current,
			schema.RequiredSchemaVersion,
		)
	}
	c.observe("schema", "succeeded")

	if c.redis != nil {
		if err := c.redis.Ping(ctx); err != nil {
			c.observe("redis", "failed")
			return fmt.Errorf("ping Redis: %w", err)
		}
		c.observe("redis", "succeeded")
	}
	return nil
}

func (c *readinessChecker) observe(dependency, result string) {
	if c.metrics != nil {
		c.metrics.ObserveReadinessCheck(dependency, result)
	}
}
