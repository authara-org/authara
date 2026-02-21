package config

import (
	"fmt"
	"time"
)

type RateLimit struct {
	LoginIPLimit        int    `env:"AUTHGATE_RATE_LIMIT_LOGIN_IP_LIMIT,default=5"`
	LoginIPWindowRaw    string `env:"AUTHGATE_RATE_LIMIT_LOGIN_IP_WINDOW,default=1m"`
	LoginEmailLimit     int    `env:"AUTHGATE_RATE_LIMIT_LOGIN_EMAIL_LIMIT,default=10"`
	LoginEmailWindowRaw string `env:"AUTHGATE_RATE_LIMIT_LOGIN_EMAIL_WINDOW,default=1h"`

	SignupIPLimit        int    `env:"AUTHGATE_RATE_LIMIT_SIGNUP_IP_LIMIT,default=3"`
	SignupIPWindowRaw    string `env:"AUTHGATE_RATE_LIMIT_SIGNUP_IP_WINDOW,default=1h"`
	SignupEmailLimit     int    `env:"AUTHGATE_RATE_LIMIT_SIGNUP_EMAIL_LIMIT,default=3"`
	SignupEmailWindowRaw string `env:"AUTHGATE_RATE_LIMIT_SIGNUP_EMAIL_WINDOW,default=24h"`

	CleanupEvery int `env:"AUTHGATE_RATE_LIMIT_CLEANUP_EVERY,default=200"`
	MaxEntries   int `env:"AUTHGATE_RATE_LIMIT_MAX_ENTRIES,default=50000"`

	LoginIPWindow     time.Duration
	LoginEmailWindow  time.Duration
	SignupIPWindow    time.Duration
	SignupEmailWindow time.Duration
}

func (r *RateLimit) validate() error {
	if r.LoginIPLimit <= 0 {
		return fmt.Errorf("AUTHGATE_RATE_LIMIT_LOGIN_IP_LIMIT must be > 0 (got %d)", r.LoginIPLimit)
	}
	if r.LoginEmailLimit <= 0 {
		return fmt.Errorf("AUTHGATE_RATE_LIMIT_LOGIN_EMAIL_LIMIT must be > 0 (got %d)", r.LoginEmailLimit)
	}
	if r.SignupIPLimit <= 0 {
		return fmt.Errorf("AUTHGATE_RATE_LIMIT_SIGNUP_IP_LIMIT must be > 0 (got %d)", r.SignupIPLimit)
	}
	if r.SignupEmailLimit <= 0 {
		return fmt.Errorf("AUTHGATE_RATE_LIMIT_SIGNUP_EMAIL_LIMIT must be > 0 (got %d)", r.SignupEmailLimit)
	}
	if r.CleanupEvery <= 0 {
		return fmt.Errorf("AUTHGATE_RATE_LIMIT_CLEANUP_EVERY must be > 0 (got %d)", r.CleanupEvery)
	}
	if r.MaxEntries <= 0 {
		return fmt.Errorf("AUTHGATE_RATE_LIMIT_MAX_ENTRIES must be > 0 (got %d)", r.MaxEntries)
	}
	return nil
}

func (r *RateLimit) parse() error {
	var err error

	r.LoginIPWindow, err = parseDurationEnv("AUTHGATE_RATE_LIMIT_LOGIN_IP_WINDOW", r.LoginIPWindowRaw)
	if err != nil {
		return err
	}
	r.LoginEmailWindow, err = parseDurationEnv("AUTHGATE_RATE_LIMIT_LOGIN_EMAIL_WINDOW", r.LoginEmailWindowRaw)
	if err != nil {
		return err
	}
	r.SignupIPWindow, err = parseDurationEnv("AUTHGATE_RATE_LIMIT_SIGNUP_IP_WINDOW", r.SignupIPWindowRaw)
	if err != nil {
		return err
	}
	r.SignupEmailWindow, err = parseDurationEnv("AUTHGATE_RATE_LIMIT_SIGNUP_EMAIL_WINDOW", r.SignupEmailWindowRaw)
	if err != nil {
		return err
	}

	if r.LoginIPWindow <= 0 {
		return fmt.Errorf("AUTHGATE_RATE_LIMIT_LOGIN_IP_WINDOW must be > 0 (got %s)", r.LoginIPWindow)
	}
	if r.LoginEmailWindow <= 0 {
		return fmt.Errorf("AUTHGATE_RATE_LIMIT_LOGIN_EMAIL_WINDOW must be > 0 (got %s)", r.LoginEmailWindow)
	}
	if r.SignupIPWindow <= 0 {
		return fmt.Errorf("AUTHGATE_RATE_LIMIT_SIGNUP_IP_WINDOW must be > 0 (got %s)", r.SignupIPWindow)
	}
	if r.SignupEmailWindow <= 0 {
		return fmt.Errorf("AUTHGATE_RATE_LIMIT_SIGNUP_EMAIL_WINDOW must be > 0 (got %s)", r.SignupEmailWindow)
	}

	return nil
}

func parseDurationEnv(varName, raw string) (time.Duration, error) {
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %q", varName, raw)
	}
	return d, nil
}
