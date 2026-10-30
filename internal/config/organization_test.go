package config

import (
	"os"
	"path/filepath"
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

func TestOrganizationLoadsInvitationDeliveryProfilesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invitation-delivery.yaml")
	if err := os.WriteFile(path, []byte(`version: 1
default: mobile
profiles:
  custom-web:
    target: external
    url: https://app.example.com/invitations/accept
    email_action: link_only
    token_transport: fragment
  mobile:
    target: external
    url: https://links.example.com/invitations/accept
    email_action: link_and_code
    token_transport: query
  manual:
    target: none
    email_action: code_only
`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Organization{InvitationTTLRaw: "24h", InvitationDeliveryProfilesFile: path}
	if err := cfg.parse(); err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultInvitationDeliveryProfile != "mobile" || cfg.InvitationDeliveryProfiles["custom-web"].TokenTransport != InvitationTokenFragment {
		t.Fatalf("profiles = %+v, default = %q", cfg.InvitationDeliveryProfiles, cfg.DefaultInvitationDeliveryProfile)
	}
	if _, ok := cfg.InvitationDeliveryProfiles[DefaultInvitationDeliveryProfile]; !ok {
		t.Fatal("built-in authara-hosted profile is missing")
	}
}

func TestOrganizationInvitationDeliveryProfilesFileIsStrict(t *testing.T) {
	tests := map[string]string{
		"unknown field": `version: 1
unknown: true
`,
		"insecure URL": `version: 1
profiles:
  mobile:
    target: external
    url: http://example.com/invitations
    email_action: link_only
`,
		"unknown default": `version: 1
default: missing
`,
		"reserved profile": `version: 1
profiles:
  authara-hosted:
    target: none
    email_action: code_only
`,
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profiles.yaml")
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg := Organization{InvitationTTLRaw: "24h", InvitationDeliveryProfilesFile: path}
			if err := cfg.parse(); err == nil {
				t.Fatal("expected invalid profile file to fail")
			}
		})
	}
}

func TestOrganizationInvitationDeliveryProfilesFileAcceptsJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"default":"manual","profiles":{"manual":{"target":"none","email_action":"code_only"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Organization{InvitationTTLRaw: "24h", InvitationDeliveryProfilesFile: path}
	if err := cfg.parse(); err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultInvitationDeliveryProfile != "manual" {
		t.Fatalf("default = %q", cfg.DefaultInvitationDeliveryProfile)
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
