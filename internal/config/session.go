package config

import "time"

type Session struct {
	SessionTTLDays          int    `env:"AUTHGATE_SESSION_TTL_DAYS" required:"true"`
	RefreshTokenTTLDays     int    `env:"AUTHGATE_REFRESH_TOKEN_TTL_DAYS" required:"true"`
	RefreshTokenRotationRaw string `env:"AUTHGATE_REFRESH_TOKEN_ROTATION_INTERVAL,default=24h"`

	SessionTTL           time.Duration
	RefreshTokenTTL      time.Duration
	RefreshTokenRotation time.Duration
}
