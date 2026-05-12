package bootstrap

import (
	"fmt"

	"github.com/authara-org/authara/internal/cache"
	"github.com/authara-org/authara/internal/config"
)

func NewCache(cfg *config.Config) (cache.Cache, error) {
	switch cfg.Cache.Provider {
	case "redis":
		return cache.NewRedis(cache.RedisConfig{
			Host:     cfg.Cache.RedisHost,
			Port:     cfg.Cache.RedisPort,
			Password: cfg.Cache.RedisPassword,
			DB:       cfg.Cache.RedisDB,
		})
	case "noop":
		return cache.NewNoop(), nil
	default:
		return nil, fmt.Errorf("unsupported cache provider %q", cfg.Cache.Provider)
	}
}
