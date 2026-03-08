package config

import (
	"fmt"
	"time"
)

type Session struct {
	SessionTTLDays          int    `env:"AUTHARA_SESSION_TTL_DAYS,default=60"`
	RefreshTokenTTLDays     int    `env:"AUTHARA_REFRESH_TOKEN_TTL_DAYS,default=14"`
	RefreshTokenRotationRaw string `env:"AUTHARA_REFRESH_TOKEN_ROTATION_INTERVAL,default=24h"`

	SessionTTL           time.Duration
	RefreshTokenTTL      time.Duration
	RefreshTokenRotation time.Duration
}

func (s *Session) validate() error {
	if s.SessionTTLDays <= 0 {
		return fmt.Errorf(
			"AUTHARA_SESSION_TTL_DAYS must be greater than 0 (got %d)",
			s.SessionTTLDays,
		)
	}

	if s.RefreshTokenTTLDays <= 0 {
		return fmt.Errorf(
			"AUTHARA_REFRESH_TOKEN_TTL_DAYS must be greater than 0 (got %d)",
			s.RefreshTokenTTLDays,
		)
	}

	if s.RefreshTokenTTLDays > s.SessionTTLDays {
		return fmt.Errorf(
			"AUTHARA_REFRESH_TOKEN_TTL_DAYS (%d) must not exceed AUTHARA_SESSION_TTL_DAYS (%d)",
			s.RefreshTokenTTLDays,
			s.SessionTTLDays,
		)
	}

	return nil
}

func (s *Session) parse() error {
	rotation, err := parseRotationInterval(s.RefreshTokenRotationRaw)
	if err != nil {
		return err
	}

	s.RefreshTokenRotation = rotation
	s.SessionTTL = time.Duration(s.SessionTTLDays) * 24 * time.Hour
	s.RefreshTokenTTL = time.Duration(s.RefreshTokenTTLDays) * 24 * time.Hour

	return nil
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
				"invalid AUTHARA_REFRESH_TOKEN_ROTATION_INTERVAL: %q",
				v,
			)
		}
		return d, nil
	}
}
