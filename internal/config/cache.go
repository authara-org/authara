package config

import (
	"fmt"
	"strings"
	"time"
)

const (
	AccessTokenRevocationModeImmediate = "immediate"
	AccessTokenRevocationModeExpiry    = "expiry"

	// ExpiryOnlyMaxAccessTokenTTL bounds the period during which a token can
	// remain usable after its backing session or authorization is revoked.
	ExpiryOnlyMaxAccessTokenTTL = 10 * time.Minute

	// MaxAccessTokenTTL is the longest access-token lifetime supported by any
	// revocation mode. Immediate-mode markers are retained for at least this
	// long so they also cover tokens issued before a policy reduction.
	MaxAccessTokenTTL = 24 * time.Hour
)

type Cache struct {
	Provider                  string `env:"AUTHARA_CACHE_PROVIDER,default=noop"`
	AccessTokenRevocationMode string `env:"AUTHARA_ACCESS_TOKEN_REVOCATION_MODE"`
	RedisHost                 string `env:"AUTHARA_REDIS_HOST,default=localhost"`
	RedisPort                 int    `env:"AUTHARA_REDIS_PORT,default=6379"`
	RedisPassword             string `env:"AUTHARA_REDIS_PASSWORD"`
	RedisDB                   int    `env:"AUTHARA_REDIS_DB,default=0"`
}

func (c *Cache) validate() error {
	c.Provider = strings.ToLower(strings.TrimSpace(c.Provider))
	c.AccessTokenRevocationMode = strings.ToLower(strings.TrimSpace(c.AccessTokenRevocationMode))

	switch c.Provider {
	case "noop", "redis":
	default:
		return fmt.Errorf("invalid AUTHARA_CACHE_PROVIDER %q (allowed: noop, redis)", c.Provider)
	}

	switch c.AccessTokenRevocationMode {
	case "", AccessTokenRevocationModeImmediate, AccessTokenRevocationModeExpiry:
	default:
		return fmt.Errorf(
			"invalid AUTHARA_ACCESS_TOKEN_REVOCATION_MODE %q (allowed: immediate, expiry)",
			c.AccessTokenRevocationMode,
		)
	}

	if c.RedisPort <= 0 || c.RedisPort > 65535 {
		return fmt.Errorf("invalid AUTHARA_REDIS_PORT %d", c.RedisPort)
	}
	if c.RedisDB < 0 {
		return fmt.Errorf("AUTHARA_REDIS_DB must be >= 0")
	}

	if c.Provider == "redis" && strings.TrimSpace(c.RedisHost) == "" {
		return fmt.Errorf("AUTHARA_REDIS_HOST must not be empty when AUTHARA_CACHE_PROVIDER=redis")
	}

	return nil
}

func (c *Cache) parse() error {
	c.Provider = strings.ToLower(strings.TrimSpace(c.Provider))
	c.AccessTokenRevocationMode = strings.ToLower(strings.TrimSpace(c.AccessTokenRevocationMode))
	c.RedisHost = strings.TrimSpace(c.RedisHost)

	return nil
}
