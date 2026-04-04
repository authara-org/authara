package config

import (
	"fmt"
	"time"
)

type Challenge struct {
	Enabled             bool          `env:"AUTHARA_CHALLENGE_ENABLED,default=false"`
	TTL                 time.Duration `env:"AUTHARA_CHALLENGE_TTL,default=30m"`
	VerificationCodeTTL time.Duration `env:"AUTHARA_CHALLENGE_VERIFICATION_CODE_TTL,default=10m"`
	MaxAttempts         int           `env:"AUTHARA_CHALLENGE_MAX_ATTEMPTS,default=5"`
	MaxResends          int           `env:"AUTHARA_CHALLENGE_MAX_RESENDS,default=3"`
	MinResendInterval   time.Duration `env:"AUTHARA_CHALLENGE_MIN_RESEND_INTERVAL,default=30s"`
}

func (c *Challenge) validate() error {
	if c.TTL <= 0 {
		return fmt.Errorf("AUTHARA_CHALLENGE_TTL must be > 0")
	}
	if c.VerificationCodeTTL <= 0 {
		return fmt.Errorf("AUTHARA_CHALLENGE_VERIFICATION_CODE_TTL must be > 0")
	}
	if c.VerificationCodeTTL > c.TTL {
		return fmt.Errorf(
			"AUTHARA_CHALLENGE_VERIFICATION_CODE_TTL (%s) must not exceed AUTHARA_CHALLENGE_TTL (%s)",
			c.VerificationCodeTTL,
			c.TTL,
		)
	}
	if c.MaxAttempts <= 0 {
		return fmt.Errorf("AUTHARA_CHALLENGE_MAX_ATTEMPTS must be > 0")
	}
	if c.MaxResends < 0 {
		return fmt.Errorf("AUTHARA_CHALLENGE_MAX_RESENDS must be >= 0")
	}
	if c.MinResendInterval < 0 {
		return fmt.Errorf("AUTHARA_CHALLENGE_MIN_RESEND_INTERVAL must be >= 0")
	}

	return nil
}
