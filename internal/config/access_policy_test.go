package config

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/sethvargo/go-envconfig"
)

func TestAccessPolicyConfigReadsBootstrapAllowedEmails(t *testing.T) {
	var cfg AccessPolicy
	if err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target: &cfg,
		Lookuper: envconfig.MapLookuper(map[string]string{
			"AUTHARA_BOOTSTRAP_ALLOWED_EMAIL": "alpha@example.com, beta@example.com",
		}),
	}); err != nil {
		t.Fatalf("process access policy config: %v", err)
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate access policy config: %v", err)
	}
	cfg.parse()

	want := []string{"alpha@example.com", "beta@example.com"}
	if !reflect.DeepEqual(cfg.BootstrapAllowedEmails, want) {
		t.Fatalf("bootstrap allowed emails = %v, want %v", cfg.BootstrapAllowedEmails, want)
	}
}

func TestAccessPolicyConfigAcceptsSingleBootstrapAllowedEmail(t *testing.T) {
	cfg := AccessPolicy{BootstrapAllowedEmails: []string{"alpha@example.com"}}
	if err := cfg.validate(); err != nil {
		t.Fatalf("single bootstrap allowed email should remain valid: %v", err)
	}
}

func TestAccessPolicyConfigRejectsInvalidBootstrapAllowedEmails(t *testing.T) {
	for _, test := range []struct {
		name   string
		emails []string
		want   string
	}{
		{name: "invalid", emails: []string{"not-an-email"}, want: "canonical email addresses"},
		{name: "display name", emails: []string{"Founder <alpha@example.com>"}, want: "canonical email addresses"},
		{name: "uppercase", emails: []string{"Alpha@example.com"}, want: "canonical email addresses"},
		{name: "duplicate", emails: []string{"alpha@example.com", "alpha@example.com"}, want: "duplicate email addresses"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := AccessPolicy{BootstrapAllowedEmails: test.emails}
			if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validate() error = %v, want %q", err, test.want)
			}
		})
	}
}
