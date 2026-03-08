package config

import (
	"fmt"
	"strings"
)

type OAuth struct {
	Providers []string `env:"AUTHARA_OAUTH_PROVIDERS"`

	GoogleClientID string `env:"AUTHARA_OAUTH_GOOGLE_CLIENT_ID"`
}

func (oa *OAuth) validate() error {
	seen := make(map[string]struct{})

	for _, raw := range oa.Providers {
		p := strings.ToLower(strings.TrimSpace(raw))

		if p == "" {
			continue
		}

		if _, ok := seen[p]; ok {
			return fmt.Errorf("duplicate OAuth provider %q", p)
		}
		seen[p] = struct{}{}

		switch p {
		case "google":
			if oa.GoogleClientID == "" {
				return fmt.Errorf("AUTHARA_OAUTH_GOOGLE_CLIENT_ID is required")
			}
		default:
			return fmt.Errorf("unsupported OAuth provider %q", p)
		}
	}
	return nil
}
