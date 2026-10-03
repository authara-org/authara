package bootstrap

import (
	"log/slog"
	"net/url"
	"strings"

	"github.com/authara-org/authara/internal/config"
)

func warnIfTransportMayBeInsecure(cfg *config.Config, logger *slog.Logger) {
	publicURL, err := url.Parse(cfg.Values.PublicURL)
	if err == nil && cfg.Values.AppEnv == "prod" && !strings.EqualFold(publicURL.Scheme, "https") {
		logger.Warn(
			"production PUBLIC_URL does not use HTTPS; authentication traffic may be exposed and secure cookies may not work",
			"scheme", publicURL.Scheme,
		)
	}

	if cfg.Cache.Provider == "redis" {
		logger.Warn(
			"Redis transport is not encrypted; use a trusted private network until Redis TLS is configured",
			"redis_host", cfg.Cache.RedisHost,
			"redis_port", cfg.Cache.RedisPort,
		)
	}
}
