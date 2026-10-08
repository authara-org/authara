package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
)

const (
	SecurityEventSelectionAll      = "all"
	SecurityEventSelectionNone     = "none"
	SecurityEventSelectionStandard = "standard"
)

type SecurityEvents struct {
	EnabledEvents   []string      `env:"AUTHARA_SECURITY_EVENT_ENABLED_EVENTS"`
	RetentionDays   int           `env:"AUTHARA_SECURITY_EVENT_RETENTION_DAYS,default=180"`
	CleanupInterval time.Duration `env:"AUTHARA_SECURITY_EVENT_CLEANUP_INTERVAL,default=24h"`

	EnabledEventSet map[domain.SecurityEventType]struct{}
}

func (s *SecurityEvents) validate() error {
	if s.RetentionDays <= 0 {
		return fmt.Errorf("AUTHARA_SECURITY_EVENT_RETENTION_DAYS must be greater than 0")
	}
	if s.CleanupInterval <= 0 {
		return fmt.Errorf("AUTHARA_SECURITY_EVENT_CLEANUP_INTERVAL must be greater than 0")
	}
	seen := make(map[string]struct{}, len(s.EnabledEvents))
	for _, raw := range s.EnabledEvents {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("duplicate security event %q", name)
		}
		seen[name] = struct{}{}
		if name == SecurityEventSelectionAll || name == SecurityEventSelectionNone || name == SecurityEventSelectionStandard {
			if len(s.EnabledEvents) != 1 {
				return fmt.Errorf("AUTHARA_SECURITY_EVENT_ENABLED_EVENTS value %q cannot be combined with event names", name)
			}
			continue
		}
		if !domain.IsSecurityEventType(name) {
			return fmt.Errorf("unsupported AUTHARA_SECURITY_EVENT_ENABLED_EVENTS value %q", name)
		}
	}
	return nil
}

func (s *SecurityEvents) parse() error {
	s.EnabledEventSet = make(map[domain.SecurityEventType]struct{})
	if len(s.EnabledEvents) == 0 {
		s.enableStandard()
		return nil
	}
	selection := strings.ToLower(strings.TrimSpace(s.EnabledEvents[0]))
	switch selection {
	case SecurityEventSelectionAll:
		for _, eventType := range domain.SecurityEventTypes() {
			s.EnabledEventSet[eventType] = struct{}{}
		}
		return nil
	case SecurityEventSelectionNone:
		return nil
	case SecurityEventSelectionStandard:
		s.enableStandard()
		return nil
	}
	for _, raw := range s.EnabledEvents {
		eventType := domain.SecurityEventType(strings.ToLower(strings.TrimSpace(raw)))
		if eventType != "" {
			s.EnabledEventSet[eventType] = struct{}{}
		}
	}
	return nil
}

func (s *SecurityEvents) EventEnabled(eventType domain.SecurityEventType) bool {
	_, enabled := s.EnabledEventSet[eventType]
	return enabled
}

func (s *SecurityEvents) enableStandard() {
	for _, eventType := range domain.StandardSecurityEventTypes() {
		s.EnabledEventSet[eventType] = struct{}{}
	}
}
