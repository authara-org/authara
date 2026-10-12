package config

import (
	"strings"
	"testing"
)

func TestValuesValidateCookieDomain(t *testing.T) {
	tests := []struct {
		name       string
		publicURL  string
		domain     string
		wantDomain string
		wantError  string
	}{
		{name: "host only", publicURL: "https://auth.example.com"},
		{
			name:       "parent domain",
			publicURL:  "https://auth.example.com",
			domain:     "example.com",
			wantDomain: "example.com",
		},
		{
			name:       "leading dot and case are normalized",
			publicURL:  "https://auth.example.com",
			domain:     ".Example.COM",
			wantDomain: "example.com",
		},
		{
			name:       "exact host",
			publicURL:  "https://auth.example.com",
			domain:     "auth.example.com",
			wantDomain: "auth.example.com",
		},
		{
			name:      "sibling domain",
			publicURL: "https://auth.example.com",
			domain:    "app.example.com",
			wantError: "must match PUBLIC_URL host",
		},
		{
			name:      "public suffix",
			publicURL: "https://auth.example.com",
			domain:    "com",
			wantError: "must be a registrable domain",
		},
		{
			name:      "localhost",
			publicURL: "http://localhost:3001",
			domain:    "localhost",
			wantError: "must be a registrable domain",
		},
		{
			name:      "surrounding whitespace",
			publicURL: "https://auth.example.com",
			domain:    " example.com ",
			wantError: "must not contain leading or trailing whitespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := Values{AppEnv: "dev", PublicURL: tt.publicURL, CookieDomain: tt.domain}

			err := values.validate()
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("validate error = %v, want substring %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("validate: %v", err)
			}
			if values.CookieDomain != tt.wantDomain {
				t.Fatalf("cookie domain = %q, want %q", values.CookieDomain, tt.wantDomain)
			}
		})
	}
}
