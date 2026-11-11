package config

import (
	"fmt"
	"net/mail"
	"strings"

	"github.com/authara-org/authara/internal/identity"
)

type AccessPolicy struct {
	AllowedEmailEnabled    bool     `env:"AUTHARA_ACCESS_POLICY_ALLOWLIST_ENABLED,default=false"`
	BootstrapAllowedEmails []string `env:"AUTHARA_BOOTSTRAP_ALLOWED_EMAIL"`
}

func (a *AccessPolicy) validate() error {
	seen := make(map[string]struct{}, len(a.BootstrapAllowedEmails))
	for _, raw := range a.BootstrapAllowedEmails {
		email := strings.TrimSpace(raw)
		address, err := mail.ParseAddress(email)
		if err != nil || address.Address != email || identity.CanonicalEmail(email) != email || strings.ContainsAny(email, "\r\n") {
			return fmt.Errorf("AUTHARA_BOOTSTRAP_ALLOWED_EMAIL must contain canonical email addresses")
		}
		if _, duplicate := seen[email]; duplicate {
			return fmt.Errorf("AUTHARA_BOOTSTRAP_ALLOWED_EMAIL must not contain duplicate email addresses")
		}
		seen[email] = struct{}{}
	}
	return nil
}

func (a *AccessPolicy) parse() {
	for i := range a.BootstrapAllowedEmails {
		a.BootstrapAllowedEmails[i] = identity.CanonicalEmail(a.BootstrapAllowedEmails[i])
	}
}
