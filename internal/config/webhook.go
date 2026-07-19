package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/webhook"
)

type Webhook struct {
	URLRaw               string        `env:"AUTHARA_WEBHOOK_URL"`
	Secret               string        `env:"AUTHARA_WEBHOOK_SECRET"`
	EnabledEvents        []string      `env:"AUTHARA_WEBHOOK_ENABLED_EVENTS"`
	Timeout              time.Duration `env:"AUTHARA_WEBHOOK_TIMEOUT,default=5s"`
	WorkerCount          int           `env:"AUTHARA_WEBHOOK_WORKER_COUNT,default=2"`
	MaxDeliveryAttempts  int           `env:"AUTHARA_WEBHOOK_MAX_DELIVERY_ATTEMPTS,default=3"`
	ProcessingStaleAfter time.Duration `env:"AUTHARA_WEBHOOK_PROCESSING_STALE_AFTER,default=2m"`
	StaleReaperInterval  time.Duration `env:"AUTHARA_WEBHOOK_STALE_REAPER_INTERVAL,default=1m"`
	DeliveredRetention   time.Duration `env:"AUTHARA_WEBHOOK_DELIVERED_RETENTION,default=24h"`
	FailedRetention      time.Duration `env:"AUTHARA_WEBHOOK_FAILED_RETENTION,default=720h"`
	CleanupInterval      time.Duration `env:"AUTHARA_WEBHOOK_CLEANUP_INTERVAL,default=1h"`
	MaintenanceBatchSize int           `env:"AUTHARA_WEBHOOK_MAINTENANCE_BATCH_SIZE,default=1000"`

	URL             string
	EnabledEventSet map[string]struct{}
}

func (w *Webhook) validate() error {
	switch {
	case w.WorkerCount <= 0:
		return fmt.Errorf("AUTHARA_WEBHOOK_WORKER_COUNT must be > 0")
	case w.MaxDeliveryAttempts <= 0:
		return fmt.Errorf("AUTHARA_WEBHOOK_MAX_DELIVERY_ATTEMPTS must be > 0")
	case w.ProcessingStaleAfter <= 0:
		return fmt.Errorf("AUTHARA_WEBHOOK_PROCESSING_STALE_AFTER must be > 0")
	case w.StaleReaperInterval <= 0:
		return fmt.Errorf("AUTHARA_WEBHOOK_STALE_REAPER_INTERVAL must be > 0")
	case w.DeliveredRetention <= 0:
		return fmt.Errorf("AUTHARA_WEBHOOK_DELIVERED_RETENTION must be > 0")
	case w.FailedRetention <= 0:
		return fmt.Errorf("AUTHARA_WEBHOOK_FAILED_RETENTION must be > 0")
	case w.CleanupInterval <= 0:
		return fmt.Errorf("AUTHARA_WEBHOOK_CLEANUP_INTERVAL must be > 0")
	case w.MaintenanceBatchSize <= 0:
		return fmt.Errorf("AUTHARA_WEBHOOK_MAINTENANCE_BATCH_SIZE must be > 0")
	case w.Timeout <= 0:
		return fmt.Errorf("AUTHARA_WEBHOOK_TIMEOUT must be > 0")
	case w.Timeout >= w.ProcessingStaleAfter:
		return fmt.Errorf("AUTHARA_WEBHOOK_TIMEOUT must be less than AUTHARA_WEBHOOK_PROCESSING_STALE_AFTER")
	}

	if (w.URLRaw == "") != (w.Secret == "") {
		return fmt.Errorf("AUTHARA_WEBHOOK_URL and AUTHARA_WEBHOOK_SECRET must be set together")
	}

	if w.URLRaw != "" {
		u, err := url.Parse(w.URLRaw)
		if err != nil {
			return fmt.Errorf("invalid AUTHARA_WEBHOOK_URL: %w", err)
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("invalid AUTHARA_WEBHOOK_URL %q: must include scheme and host", w.URLRaw)
		}
	}

	seen := make(map[string]struct{})
	for _, raw := range w.EnabledEvents {
		ev := strings.TrimSpace(strings.ToLower(raw))
		if ev == "" {
			continue
		}
		if _, ok := seen[ev]; ok {
			return fmt.Errorf("duplicate webhook event %q", ev)
		}
		seen[ev] = struct{}{}

		if !supportedWebhookEvent(ev) {
			return fmt.Errorf("unsupported AUTHARA_WEBHOOK_ENABLED_EVENTS value %q", ev)
		}
	}

	return nil
}

func supportedWebhookEvent(name string) bool {
	for _, eventType := range webhook.SupportedEventTypes {
		if name == string(eventType) {
			return true
		}
	}
	return false
}

func (w *Webhook) parse() error {
	w.URL = strings.TrimSpace(w.URLRaw)
	w.EnabledEventSet = make(map[string]struct{})

	for _, raw := range w.EnabledEvents {
		ev := strings.TrimSpace(strings.ToLower(raw))
		if ev == "" {
			continue
		}
		w.EnabledEventSet[ev] = struct{}{}
	}

	return nil
}

func (w *Webhook) Enabled() bool {
	return w.URL != "" && w.Secret != ""
}

func (w *Webhook) EventEnabled(name string) bool {
	if !w.Enabled() {
		return false
	}

	// If no explicit list → all events enabled
	if len(w.EnabledEventSet) == 0 {
		return true
	}

	_, ok := w.EnabledEventSet[name]
	return ok
}
