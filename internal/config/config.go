package config

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	Values       Values
	DB           DB
	Cache        Cache
	Logging      Logging
	OAuth        OAuth
	Token        Token
	Session      Session
	RateLimit    RateLimit
	Webhook      Webhook
	AccessPolicy AccessPolicy
	Challenge    Challenge
	Email        Email
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
	if err := cfg.Cache.validate(); err != nil {
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
	if err := cfg.Webhook.validate(); err != nil {
		return nil, err
	}
	if err := cfg.AccessPolicy.validate(); err != nil {
		return nil, err
	}
	if err := cfg.Challenge.validate(); err != nil {
		return nil, err
	}
	if err := cfg.Email.validate(); err != nil {
		return nil, err
	}

	cfg.Values.HttpAddr = ":8080"

	if err := cfg.Logging.parse(cfg.Values.AppEnv); err != nil {
		return nil, err
	}
	if err := cfg.Cache.parse(); err != nil {
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
	if err := cfg.Webhook.parse(); err != nil {
		return nil, err
	}
	if err := cfg.Email.parse(); err != nil {
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

	if c.Challenge.Enabled &&
		c.Email.Provider == "noop" &&
		c.Values.AppEnv == "prod" {
		return fmt.Errorf("AUTHARA_EMAIL_PROVIDER must not be noop when AUTHARA_CHALLENGE_ENABLED=true in production")
	}

	if c.Values.AppEnv == "prod" && c.DB.LogSQL {
		return fmt.Errorf("POSTGRESQL_LOG_SQL must be false when APP_ENV=prod")
	}

	if c.Values.AppEnv == "prod" && c.Email.Provider == "smtp" && !c.Email.SMTPTLS {
		return fmt.Errorf("AUTHARA_EMAIL_SMTP_TLS must be true when APP_ENV=prod and AUTHARA_EMAIL_PROVIDER=smtp")
	}

	if c.Values.AppEnv == "prod" && c.Webhook.Enabled() {
		if len(c.Webhook.Secret) < 32 {
			return fmt.Errorf("AUTHARA_WEBHOOK_SECRET must be at least 32 characters when APP_ENV=prod")
		}
	}

	return nil
}
