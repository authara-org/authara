package config

import (
	"testing"
	"time"
)

func TestOrganizationValidate(t *testing.T) {
	for _, mode := range []string{OrgModePersonal, OrgModeSingle, OrgModeMulti} {
		t.Run(mode, func(t *testing.T) {
			if err := (&Organization{Mode: mode}).validate(); err != nil {
				t.Fatalf("validate failed: %v", err)
			}
		})
	}

	if err := (&Organization{Mode: "invalid"}).validate(); err == nil {
		t.Fatal("expected invalid org mode to fail")
	}
}

func TestOrganizationParseInvitationTTL(t *testing.T) {
	var cfg Organization
	cfg.InvitationTTLRaw = "24h"
	if err := cfg.parse(); err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if cfg.InvitationTTL != 24*time.Hour {
		t.Fatalf("expected 24h TTL, got %s", cfg.InvitationTTL)
	}

	cfg.InvitationTTLRaw = "0"
	if err := cfg.parse(); err == nil {
		t.Fatal("expected zero TTL to fail")
	}
}
