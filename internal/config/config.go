package config

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	Values    Values
	DB        DB
	Logging   Logging
	OAuth     OAuth
	Token     Token
	Session   Session
	RateLimit RateLimit
}

func Load() (*Config, error) {
	var cfg Config

	if err := envconfig.Process(context.Background(), &cfg); err != nil {
		return nil, err
	}

	if err := cfg.Values.validate(); err != nil {
		return nil, err
	}
	if err := cfg.DB.validate(); err != nil {
		return nil, err
	}
	if err := cfg.Logging.validate(); err != nil {
		return nil, err
	}
	if err := cfg.OAuth.validate(); err != nil {
		return nil, err
	}
	if err := cfg.Token.validate(); err != nil {
		return nil, err
	}
	if err := cfg.Session.validate(); err != nil {
		return nil, err
	}
	if err := cfg.RateLimit.validate(); err != nil {
		return nil, err
	}

	cfg.Values.HttpAddr = ":8080"

	if err := cfg.Logging.parse(cfg.Values.AppEnv); err != nil {
		return nil, err
	}
	if err := cfg.Token.parse(); err != nil {
		return nil, err
	}
	if err := cfg.Session.parse(); err != nil {
		return nil, err
	}
	if err := cfg.RateLimit.parse(); err != nil {
		return nil, err
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Token.AccessTokenTTL >= c.Session.RefreshTokenTTL {
		return fmt.Errorf(
			"invalid token configuration: access token TTL (%s) "+
				"must be less than refresh token TTL (%s)",
			c.Token.AccessTokenTTL,
			c.Session.RefreshTokenTTL,
		)
	}
	return nil
}
