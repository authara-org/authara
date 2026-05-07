package config

import (
	"fmt"
	"time"
)

type RateLimit struct {
	LoginIPLimit        int    `env:"AUTHARA_RATE_LIMIT_LOGIN_IP_LIMIT,default=5"`
	LoginIPWindowRaw    string `env:"AUTHARA_RATE_LIMIT_LOGIN_IP_WINDOW,default=1m"`
	LoginEmailLimit     int    `env:"AUTHARA_RATE_LIMIT_LOGIN_EMAIL_LIMIT,default=10"`
	LoginEmailWindowRaw string `env:"AUTHARA_RATE_LIMIT_LOGIN_EMAIL_WINDOW,default=1h"`

	SignupIPLimit        int    `env:"AUTHARA_RATE_LIMIT_SIGNUP_IP_LIMIT,default=3"`
	SignupIPWindowRaw    string `env:"AUTHARA_RATE_LIMIT_SIGNUP_IP_WINDOW,default=1h"`
	SignupEmailLimit     int    `env:"AUTHARA_RATE_LIMIT_SIGNUP_EMAIL_LIMIT,default=3"`
	SignupEmailWindowRaw string `env:"AUTHARA_RATE_LIMIT_SIGNUP_EMAIL_WINDOW,default=24h"`

	PasswordResetIPLimit        int    `env:"AUTHARA_RATE_LIMIT_PASSWORD_RESET_IP_LIMIT,default=5"`
	PasswordResetIPWindowRaw    string `env:"AUTHARA_RATE_LIMIT_PASSWORD_RESET_IP_WINDOW,default=1h"`
	PasswordResetEmailLimit     int    `env:"AUTHARA_RATE_LIMIT_PASSWORD_RESET_EMAIL_LIMIT,default=3"`
	PasswordResetEmailWindowRaw string `env:"AUTHARA_RATE_LIMIT_PASSWORD_RESET_EMAIL_WINDOW,default=24h"`

	ChallengeVerifyIPLimit     int    `env:"AUTHARA_RATE_LIMIT_CHALLENGE_VERIFY_IP_LIMIT,default=30"`
	ChallengeVerifyIPWindowRaw string `env:"AUTHARA_RATE_LIMIT_CHALLENGE_VERIFY_IP_WINDOW,default=10m"`
	ChallengeVerifyIDLimit     int    `env:"AUTHARA_RATE_LIMIT_CHALLENGE_VERIFY_ID_LIMIT,default=10"`
	ChallengeVerifyIDWindowRaw string `env:"AUTHARA_RATE_LIMIT_CHALLENGE_VERIFY_ID_WINDOW,default=30m"`

	ChallengeResendIPLimit     int    `env:"AUTHARA_RATE_LIMIT_CHALLENGE_RESEND_IP_LIMIT,default=10"`
	ChallengeResendIPWindowRaw string `env:"AUTHARA_RATE_LIMIT_CHALLENGE_RESEND_IP_WINDOW,default=1h"`
	ChallengeResendIDLimit     int    `env:"AUTHARA_RATE_LIMIT_CHALLENGE_RESEND_ID_LIMIT,default=5"`
	ChallengeResendIDWindowRaw string `env:"AUTHARA_RATE_LIMIT_CHALLENGE_RESEND_ID_WINDOW,default=30m"`

	CleanupEvery int `env:"AUTHARA_RATE_LIMIT_CLEANUP_EVERY,default=200"`
	MaxEntries   int `env:"AUTHARA_RATE_LIMIT_MAX_ENTRIES,default=50000"`

	LoginIPWindow            time.Duration
	LoginEmailWindow         time.Duration
	SignupIPWindow           time.Duration
	SignupEmailWindow        time.Duration
	PasswordResetIPWindow    time.Duration
	PasswordResetEmailWindow time.Duration
	ChallengeVerifyIPWindow  time.Duration
	ChallengeVerifyIDWindow  time.Duration
	ChallengeResendIPWindow  time.Duration
	ChallengeResendIDWindow  time.Duration
}

func (r *RateLimit) validate() error {
	if r.LoginIPLimit <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_LOGIN_IP_LIMIT must be > 0 (got %d)", r.LoginIPLimit)
	}
	if r.LoginEmailLimit <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_LOGIN_EMAIL_LIMIT must be > 0 (got %d)", r.LoginEmailLimit)
	}
	if r.SignupIPLimit <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_SIGNUP_IP_LIMIT must be > 0 (got %d)", r.SignupIPLimit)
	}
	if r.SignupEmailLimit <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_SIGNUP_EMAIL_LIMIT must be > 0 (got %d)", r.SignupEmailLimit)
	}
	if r.PasswordResetIPLimit <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_PASSWORD_RESET_IP_LIMIT must be > 0 (got %d)", r.PasswordResetIPLimit)
	}
	if r.PasswordResetEmailLimit <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_PASSWORD_RESET_EMAIL_LIMIT must be > 0 (got %d)", r.PasswordResetEmailLimit)
	}
	if r.ChallengeVerifyIPLimit <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_CHALLENGE_VERIFY_IP_LIMIT must be > 0 (got %d)", r.ChallengeVerifyIPLimit)
	}
	if r.ChallengeVerifyIDLimit <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_CHALLENGE_VERIFY_ID_LIMIT must be > 0 (got %d)", r.ChallengeVerifyIDLimit)
	}
	if r.ChallengeResendIPLimit <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_CHALLENGE_RESEND_IP_LIMIT must be > 0 (got %d)", r.ChallengeResendIPLimit)
	}
	if r.ChallengeResendIDLimit <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_CHALLENGE_RESEND_ID_LIMIT must be > 0 (got %d)", r.ChallengeResendIDLimit)
	}
	if r.CleanupEvery <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_CLEANUP_EVERY must be > 0 (got %d)", r.CleanupEvery)
	}
	if r.MaxEntries <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_MAX_ENTRIES must be > 0 (got %d)", r.MaxEntries)
	}
	return nil
}

func (r *RateLimit) parse() error {
	var err error

	r.LoginIPWindow, err = parseDurationEnv("AUTHARA_RATE_LIMIT_LOGIN_IP_WINDOW", r.LoginIPWindowRaw)
	if err != nil {
		return err
	}
	r.LoginEmailWindow, err = parseDurationEnv("AUTHARA_RATE_LIMIT_LOGIN_EMAIL_WINDOW", r.LoginEmailWindowRaw)
	if err != nil {
		return err
	}
	r.SignupIPWindow, err = parseDurationEnv("AUTHARA_RATE_LIMIT_SIGNUP_IP_WINDOW", r.SignupIPWindowRaw)
	if err != nil {
		return err
	}
	r.SignupEmailWindow, err = parseDurationEnv("AUTHARA_RATE_LIMIT_SIGNUP_EMAIL_WINDOW", r.SignupEmailWindowRaw)
	if err != nil {
		return err
	}
	r.PasswordResetIPWindow, err = parseDurationEnv("AUTHARA_RATE_LIMIT_PASSWORD_RESET_IP_WINDOW", r.PasswordResetIPWindowRaw)
	if err != nil {
		return err
	}
	r.PasswordResetEmailWindow, err = parseDurationEnv("AUTHARA_RATE_LIMIT_PASSWORD_RESET_EMAIL_WINDOW", r.PasswordResetEmailWindowRaw)
	if err != nil {
		return err
	}
	r.ChallengeVerifyIPWindow, err = parseDurationEnv("AUTHARA_RATE_LIMIT_CHALLENGE_VERIFY_IP_WINDOW", r.ChallengeVerifyIPWindowRaw)
	if err != nil {
		return err
	}
	r.ChallengeVerifyIDWindow, err = parseDurationEnv("AUTHARA_RATE_LIMIT_CHALLENGE_VERIFY_ID_WINDOW", r.ChallengeVerifyIDWindowRaw)
	if err != nil {
		return err
	}
	r.ChallengeResendIPWindow, err = parseDurationEnv("AUTHARA_RATE_LIMIT_CHALLENGE_RESEND_IP_WINDOW", r.ChallengeResendIPWindowRaw)
	if err != nil {
		return err
	}
	r.ChallengeResendIDWindow, err = parseDurationEnv("AUTHARA_RATE_LIMIT_CHALLENGE_RESEND_ID_WINDOW", r.ChallengeResendIDWindowRaw)
	if err != nil {
		return err
	}

	if r.LoginIPWindow <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_LOGIN_IP_WINDOW must be > 0 (got %s)", r.LoginIPWindow)
	}
	if r.LoginEmailWindow <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_LOGIN_EMAIL_WINDOW must be > 0 (got %s)", r.LoginEmailWindow)
	}
	if r.SignupIPWindow <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_SIGNUP_IP_WINDOW must be > 0 (got %s)", r.SignupIPWindow)
	}
	if r.SignupEmailWindow <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_SIGNUP_EMAIL_WINDOW must be > 0 (got %s)", r.SignupEmailWindow)
	}
	if r.PasswordResetIPWindow <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_PASSWORD_RESET_IP_WINDOW must be > 0 (got %s)", r.PasswordResetIPWindow)
	}
	if r.PasswordResetEmailWindow <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_PASSWORD_RESET_EMAIL_WINDOW must be > 0 (got %s)", r.PasswordResetEmailWindow)
	}
	if r.ChallengeVerifyIPWindow <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_CHALLENGE_VERIFY_IP_WINDOW must be > 0 (got %s)", r.ChallengeVerifyIPWindow)
	}
	if r.ChallengeVerifyIDWindow <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_CHALLENGE_VERIFY_ID_WINDOW must be > 0 (got %s)", r.ChallengeVerifyIDWindow)
	}
	if r.ChallengeResendIPWindow <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_CHALLENGE_RESEND_IP_WINDOW must be > 0 (got %s)", r.ChallengeResendIPWindow)
	}
	if r.ChallengeResendIDWindow <= 0 {
		return fmt.Errorf("AUTHARA_RATE_LIMIT_CHALLENGE_RESEND_ID_WINDOW must be > 0 (got %s)", r.ChallengeResendIDWindow)
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
