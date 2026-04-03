package config

import (
	"fmt"
	"strings"
	"time"
)

type Email struct {
	Enabled bool `env:"AUTHARA_EMAIL_ENABLED,default=false"`

	Provider string `env:"AUTHARA_EMAIL_PROVIDER,default=noop"`
	From     string `env:"AUTHARA_EMAIL_FROM"`

	SMTPHost     string        `env:"AUTHARA_EMAIL_SMTP_HOST"`
	SMTPPort     int           `env:"AUTHARA_EMAIL_SMTP_PORT,default=587"`
	SMTPUsername string        `env:"AUTHARA_EMAIL_SMTP_USERNAME"`
	SMTPPassword string        `env:"AUTHARA_EMAIL_SMTP_PASSWORD"`
	SMTPTLS      bool          `env:"AUTHARA_EMAIL_SMTP_TLS,default=true"`
	SMTPTimeout  time.Duration `env:"AUTHARA_EMAIL_SMTP_TIMEOUT,default=10s"`

	WorkersEnabled       bool          `env:"AUTHARA_EMAIL_WORKERS_ENABLED,default=true"`
	WorkerCount          int           `env:"AUTHARA_EMAIL_WORKER_COUNT,default=1"`
	WorkerPollInterval   time.Duration `env:"AUTHARA_EMAIL_WORKER_POLL_INTERVAL,default=2s"`
	JobProcessingTimeout time.Duration `env:"AUTHARA_EMAIL_JOB_PROCESSING_TIMEOUT,default=2m"`
	JobMaxAttempts       int           `env:"AUTHARA_EMAIL_JOB_MAX_ATTEMPTS,default=10"`
	CleanupSentAfter     time.Duration `env:"AUTHARA_EMAIL_CLEANUP_SENT_AFTER,default=720h"`    // 30d
	CleanupFailedAfter   time.Duration `env:"AUTHARA_EMAIL_CLEANUP_FAILED_AFTER,default=2160h"` // 90d
}

func (e *Email) validate() error {
	e.Provider = strings.ToLower(strings.TrimSpace(e.Provider))

	switch e.Provider {
	case "noop", "smtp":
	default:
		return fmt.Errorf("invalid AUTHARA_EMAIL_PROVIDER %q (allowed: noop, smtp, api)", e.Provider)
	}

	if !e.Enabled {
		return nil
	}

	if e.From == "" {
		return fmt.Errorf("AUTHARA_EMAIL_FROM must not be empty when AUTHARA_EMAIL_ENABLED=true")
	}

	switch e.Provider {
	case "smtp":
		if e.SMTPHost == "" {
			return fmt.Errorf("AUTHARA_EMAIL_SMTP_HOST must not be empty when AUTHARA_EMAIL_PROVIDER=smtp")
		}
		if e.SMTPPort <= 0 || e.SMTPPort > 65535 {
			return fmt.Errorf("invalid AUTHARA_EMAIL_SMTP_PORT %d", e.SMTPPort)
		}
		if e.SMTPTimeout <= 0 {
			return fmt.Errorf("AUTHARA_EMAIL_SMTP_TIMEOUT must be > 0")
		}
	}

	if e.WorkerCount <= 0 {
		return fmt.Errorf("AUTHARA_EMAIL_WORKER_COUNT must be > 0")
	}
	if e.WorkerPollInterval <= 0 {
		return fmt.Errorf("AUTHARA_EMAIL_WORKER_POLL_INTERVAL must be > 0")
	}
	if e.JobProcessingTimeout <= 0 {
		return fmt.Errorf("AUTHARA_EMAIL_JOB_PROCESSING_TIMEOUT must be > 0")
	}
	if e.JobMaxAttempts <= 0 {
		return fmt.Errorf("AUTHARA_EMAIL_JOB_MAX_ATTEMPTS must be > 0")
	}
	if e.CleanupSentAfter <= 0 {
		return fmt.Errorf("AUTHARA_EMAIL_CLEANUP_SENT_AFTER must be > 0")
	}
	if e.CleanupFailedAfter <= 0 {
		return fmt.Errorf("AUTHARA_EMAIL_CLEANUP_FAILED_AFTER must be > 0")
	}

	return nil
}

func (e *Email) parse() error {
	e.Provider = strings.ToLower(strings.TrimSpace(e.Provider))
	e.From = strings.TrimSpace(e.From)
	e.SMTPHost = strings.TrimSpace(e.SMTPHost)
	e.SMTPUsername = strings.TrimSpace(e.SMTPUsername)

	return nil
}
