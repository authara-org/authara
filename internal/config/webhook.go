package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Webhook struct {
	URLRaw        string   `env:"AUTHARA_WEBHOOK_URL"`
	Secret        string   `env:"AUTHARA_WEBHOOK_SECRET"`
	EnabledEvents []string `env:"AUTHARA_WEBHOOK_ENABLED_EVENTS"`
	TimeoutRaw    string   `env:"AUTHARA_WEBHOOK_TIMEOUT,default=5s"`

	URL             string
	Timeout         time.Duration
	EnabledEventSet map[string]struct{}
}

func (w *Webhook) validate() error {
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

		switch ev {
		case "user.created", "user.deleted":
		default:
			return fmt.Errorf("unsupported AUTHARA_WEBHOOK_ENABLED_EVENTS value %q", ev)
		}
	}

	return nil
}

func (w *Webhook) parse() error {
	timeout, err := time.ParseDuration(w.TimeoutRaw)
	if err != nil {
		return fmt.Errorf("invalid AUTHARA_WEBHOOK_TIMEOUT: %q", w.TimeoutRaw)
	}
	if timeout <= 0 {
		return fmt.Errorf("AUTHARA_WEBHOOK_TIMEOUT must be greater than 0")
	}

	w.Timeout = timeout
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
