package config

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	Values         Values
	DB             DB
	Cache          Cache
	Logging        Logging
	Observability  Observability
	UI             UI
	OAuth          OAuth
	Token          Token
	Session        Session
	RateLimit      RateLimit
	Webhook        Webhook
	AccessPolicy   AccessPolicy
	Admin          Admin
	InternalAPI    InternalAPI
	Organization   Organization
	Authentication Authentication
	SecurityEvents SecurityEvents
	Challenge      Challenge
	Email          Email
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
	if err := cfg.UI.validate(); err != nil {
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
	if err := cfg.Admin.validate(); err != nil {
		return nil, err
	}
	if err := cfg.InternalAPI.validate(); err != nil {
		return nil, err
	}
	if err := cfg.Organization.validate(); err != nil {
		return nil, err
	}
	if err := cfg.Authentication.validate(); err != nil {
		return nil, err
	}
	if err := cfg.SecurityEvents.validate(); err != nil {
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
	if err := cfg.Organization.parse(); err != nil {
		return nil, err
	}
	if err := cfg.SecurityEvents.parse(); err != nil {
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

	if err := c.validateAccessTokenRevocation(); err != nil {
		return err
	}

	if c.Values.AppEnv == "prod" && !c.Email.IsDeliverable() {
		return fmt.Errorf("AUTHARA_EMAIL_PROVIDER must be smtp in production because password recovery routes are enabled")
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

	if c.Values.AppEnv == "prod" {
		internalAPIToken := strings.TrimSpace(c.InternalAPI.Token)
		if utf8.RuneCountInString(internalAPIToken) < 24 {
			return fmt.Errorf("AUTHARA_INTERNAL_API_TOKEN must be at least 24 characters when APP_ENV=prod")
		}
		if internalAPIToken != c.InternalAPI.Token {
			return fmt.Errorf("AUTHARA_INTERNAL_API_TOKEN must not contain leading or trailing whitespace")
		}
	}

	return nil
}

func (c *Config) validateAccessTokenRevocation() error {
	if c.Token.AccessTokenTTL > MaxAccessTokenTTL {
		return fmt.Errorf(
			"AUTHARA_ACCESS_TOKEN_TTL_MINUTES must be at most %d",
			int(MaxAccessTokenTTL/time.Minute),
		)
	}

	mode := c.Cache.AccessTokenRevocationMode
	if mode == "" {
		switch {
		case c.Cache.Provider == "redis":
			mode = AccessTokenRevocationModeImmediate
		case c.Values.AppEnv == "dev" && c.Cache.Provider == "noop":
			mode = AccessTokenRevocationModeExpiry
		default:
			return fmt.Errorf(
				"AUTHARA_ACCESS_TOKEN_REVOCATION_MODE must be set to expiry when APP_ENV=prod and AUTHARA_CACHE_PROVIDER=noop",
			)
		}
		c.Cache.AccessTokenRevocationMode = mode
	}

	switch mode {
	case AccessTokenRevocationModeImmediate:
		if c.Cache.Provider != "redis" {
			return fmt.Errorf(
				"AUTHARA_ACCESS_TOKEN_REVOCATION_MODE=immediate requires AUTHARA_CACHE_PROVIDER=redis",
			)
		}
	case AccessTokenRevocationModeExpiry:
		if c.Cache.Provider != "noop" {
			return fmt.Errorf(
				"AUTHARA_ACCESS_TOKEN_REVOCATION_MODE=expiry requires AUTHARA_CACHE_PROVIDER=noop",
			)
		}
		if c.Token.AccessTokenTTL > ExpiryOnlyMaxAccessTokenTTL {
			return fmt.Errorf(
				"AUTHARA_ACCESS_TOKEN_TTL_MINUTES must be at most %d when AUTHARA_ACCESS_TOKEN_REVOCATION_MODE=expiry",
				int(ExpiryOnlyMaxAccessTokenTTL/time.Minute),
			)
		}
	}

	return nil
}
