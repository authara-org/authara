package config

import (
	"context"
	"fmt"
	"time"

	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	Env string `env:"ENV,default=dev"`
	Dev bool   `env:"DEV_MODE,default=true"`

	DB      DB
	HTTP    HTTP
	Logging Logging

	Google  Google
	Token   Token
	Session Session
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process(context.Background(), &cfg); err != nil {
		return nil, err
	}

	rotation, err := parseRotationInterval(cfg.Session.RefreshTokenRotationRaw)
	if err != nil {
		return nil, err
	}
	cfg.Session.RefreshTokenRotation = rotation

	cfg.Token.AccessTokenTTL = time.Duration(cfg.Token.AccessTokenTTLMinutes) * time.Minute
	cfg.Session.SessionTTL = time.Duration(cfg.Session.SessionTTLDays) * 24 * time.Hour
	cfg.Session.RefreshTokenTTL = time.Duration(cfg.Session.RefreshTokenTTLDays) * 24 * time.Hour

	if cfg.Session.RefreshTokenTTL > cfg.Session.SessionTTL {
		return nil, fmt.Errorf(
			"invalid configuration: AUTHGATE_REFRESH_TOKEN_TTL_DAYS (%d) "+
				"must not exceed AUTHGATE_SESSION_TTL_DAYS (%d)",
			cfg.Session.RefreshTokenTTLDays,
			cfg.Session.SessionTTLDays,
		)
	}

	return &cfg, nil
}

func parseRotationInterval(v string) (time.Duration, error) {
	switch v {
	case "", "0", "disabled", "off":
		return 0, nil
	case "always":
		return -1, nil
	default:
		d, err := time.ParseDuration(v)
		if err != nil {
			return 0, fmt.Errorf(
				"invalid AUTHGATE_REFRESH_TOKEN_ROTATION_INTERVAL: %q",
				v,
			)
		}
		return d, nil
	}
}
