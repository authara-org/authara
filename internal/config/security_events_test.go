package config

import (
	"testing"

	"github.com/authara-org/authara/internal/domain"
)

func TestSecurityEventsDefaultExcludesRoutineSessionNoise(t *testing.T) {
	cfg := SecurityEvents{RetentionDays: 180}
	if err := cfg.validate(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.parse(); err != nil {
		t.Fatal(err)
	}
	if !cfg.EventEnabled(domain.SecurityEventSessionRefreshTokenReuse) || !cfg.EventEnabled(domain.SecurityEventAuthenticationLogin) {
		t.Fatal("standard security events were not enabled")
	}
	if cfg.EventEnabled(domain.SecurityEventSessionRefresh) || cfg.EventEnabled(domain.SecurityEventSessionLogout) {
		t.Fatal("standard selection enabled routine session noise")
	}
}

func TestSecurityEventsExactSelectionAndNone(t *testing.T) {
	cfg := SecurityEvents{RetentionDays: 30, EnabledEvents: []string{string(domain.SecurityEventSessionLogout)}}
	if err := cfg.validate(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.parse(); err != nil {
		t.Fatal(err)
	}
	if !cfg.EventEnabled(domain.SecurityEventSessionLogout) || cfg.EventEnabled(domain.SecurityEventAuthenticationLogin) {
		t.Fatalf("unexpected enabled set: %#v", cfg.EnabledEventSet)
	}

	cfg = SecurityEvents{RetentionDays: 30, EnabledEvents: []string{SecurityEventSelectionNone}}
	if err := cfg.validate(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.parse(); err != nil {
		t.Fatal(err)
	}
	if len(cfg.EnabledEventSet) != 0 {
		t.Fatalf("none enabled events: %#v", cfg.EnabledEventSet)
	}
}

func TestSecurityEventsRejectsUnknownDuplicateAndMixedSelections(t *testing.T) {
	for _, events := range [][]string{
		{"unknown"},
		{string(domain.SecurityEventAuthenticationLogin), string(domain.SecurityEventAuthenticationLogin)},
		{SecurityEventSelectionAll, string(domain.SecurityEventSessionLogout)},
	} {
		cfg := SecurityEvents{RetentionDays: 30, EnabledEvents: events}
		if err := cfg.validate(); err == nil {
			t.Fatalf("selection %#v was accepted", events)
		}
	}
}
