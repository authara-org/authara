package config

import (
	"fmt"
	"strings"
)

type Cache struct {
	Provider      string `env:"AUTHARA_CACHE_PROVIDER,default=noop"`
	RedisHost     string `env:"AUTHARA_REDIS_HOST,default=localhost"`
	RedisPort     int    `env:"AUTHARA_REDIS_PORT,default=6379"`
	RedisPassword string `env:"AUTHARA_REDIS_PASSWORD"`
	RedisDB       int    `env:"AUTHARA_REDIS_DB,default=0"`
}

func (c *Cache) validate() error {
	c.Provider = strings.ToLower(strings.TrimSpace(c.Provider))

	switch c.Provider {
	case "noop", "redis":
	default:
		return fmt.Errorf("invalid AUTHARA_CACHE_PROVIDER %q (allowed: noop, redis)", c.Provider)
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
	c.RedisHost = strings.TrimSpace(c.RedisHost)

	return nil
}
