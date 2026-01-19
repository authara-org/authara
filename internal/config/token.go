package config

import "time"

type Token struct {
	Issuer                string            `env:"AUTHGATE_JWT_ISSUER" required:"true"`
	ActiveKeyID           string            `env:"AUTHGATE_JWT_ACTIVE_KEY_ID" required:"true"`
	Keys                  map[string]string `env:"AUTHGATE_JWT_KEYS" required:"true"`
	AccessTokenTTLMinutes int               `env:"AUTHGATE_ACCESS_TOKEN_TTL_MINUTES" required:"true"`

	AccessTokenTTL time.Duration
}
