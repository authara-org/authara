package config

import (
	"strings"
	"testing"
	"time"
)

func validProdConfigForValidate() Config {
	return Config{
		Values: Values{
			AppEnv: "prod",
		},
		Token: Token{
			AccessTokenTTL: time.Minute,
		},
		Cache: Cache{
			Provider:                  "noop",
			AccessTokenRevocationMode: AccessTokenRevocationModeExpiry,
		},
		Session: Session{
			RefreshTokenTTL: time.Hour,
		},
		Admin: Admin{
			AuditRetentionDays: 180,
		},
		InternalAPI: InternalAPI{
			Token: strings.Repeat("i", 24),
		},
		Email: Email{
			Provider: "smtp",
			SMTPTLS:  true,
		},
	}
}

func TestConfigValidate_AccessTokenRevocationProfiles(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		provider    string
		mode        string
		accessTTL   time.Duration
		wantMode    string
		wantError   string
	}{
		{
			name: "redis infers immediate", environment: "prod", provider: "redis",
			accessTTL: 30 * time.Minute, wantMode: AccessTokenRevocationModeImmediate,
		},
		{
			name: "immediate accepts maximum token lifetime", environment: "prod", provider: "redis",
			mode: AccessTokenRevocationModeImmediate, accessTTL: MaxAccessTokenTTL,
			wantMode: AccessTokenRevocationModeImmediate,
		},
		{
			name: "immediate rejects token lifetime above maximum", environment: "prod", provider: "redis",
			mode: AccessTokenRevocationModeImmediate, accessTTL: MaxAccessTokenTTL + time.Minute,
			wantError: "must be at most 1440",
		},
		{
			name: "development noop infers expiry", environment: "dev", provider: "noop",
			accessTTL: ExpiryOnlyMaxAccessTokenTTL, wantMode: AccessTokenRevocationModeExpiry,
		},
		{
			name: "production noop requires explicit acknowledgement", environment: "prod", provider: "noop",
			accessTTL: time.Minute, wantError: "must be set to expiry",
		},
		{
			name: "explicit production expiry", environment: "prod", provider: "noop",
			mode: AccessTokenRevocationModeExpiry, accessTTL: ExpiryOnlyMaxAccessTokenTTL,
			wantMode: AccessTokenRevocationModeExpiry,
		},
		{
			name: "expiry rejects long token", environment: "prod", provider: "noop",
			mode: AccessTokenRevocationModeExpiry, accessTTL: ExpiryOnlyMaxAccessTokenTTL + time.Minute,
			wantError: "must be at most 10",
		},
		{
			name: "immediate requires redis", environment: "prod", provider: "noop",
			mode: AccessTokenRevocationModeImmediate, accessTTL: time.Minute,
			wantError: "requires AUTHARA_CACHE_PROVIDER=redis",
		},
		{
			name: "expiry requires noop", environment: "prod", provider: "redis",
			mode: AccessTokenRevocationModeExpiry, accessTTL: time.Minute,
			wantError: "requires AUTHARA_CACHE_PROVIDER=noop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validProdConfigForValidate()
			cfg.Values.AppEnv = tt.environment
			cfg.Cache.Provider = tt.provider
			cfg.Cache.AccessTokenRevocationMode = tt.mode
			cfg.Token.AccessTokenTTL = tt.accessTTL
			if cfg.Session.RefreshTokenTTL <= tt.accessTTL {
				cfg.Session.RefreshTokenTTL = tt.accessTTL + time.Hour
			}

			err := cfg.validate()
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("validate error = %v, want substring %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("validate failed: %v", err)
			}
			if cfg.Cache.AccessTokenRevocationMode != tt.wantMode {
				t.Fatalf("mode = %q, want %q", cfg.Cache.AccessTokenRevocationMode, tt.wantMode)
			}
		})
	}
}

func TestConfigValidate_ProdRequiresDeliverableEmailForPasswordRecovery(t *testing.T) {
	cfg := validProdConfigForValidate()
	cfg.Challenge.Enabled = false
	cfg.Email.Provider = "noop"

	err := cfg.validate()
	if err == nil || !strings.Contains(err.Error(), "password recovery routes are enabled") {
		t.Fatalf("validate error = %v, want password recovery email delivery error", err)
	}
}

func TestConfigValidate_ProdAllowsSMTPWhenChallengesAreDisabled(t *testing.T) {
	cfg := validProdConfigForValidate()
	cfg.Challenge.Enabled = false

	if err := cfg.validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestConfigValidate_DevAllowsNoopEmail(t *testing.T) {
	cfg := validProdConfigForValidate()
	cfg.Values.AppEnv = "dev"
	cfg.Email.Provider = "noop"

	if err := cfg.validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestConfigValidate_ProdWebhookAllowsHTTPSURL(t *testing.T) {
	cfg := validProdConfigForValidate()
	cfg.Webhook.URL = "https://example.com/webhooks/authara"
	cfg.Webhook.Secret = strings.Repeat("s", 32)

	if err := cfg.validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestConfigValidate_ProdWebhookAllowsHTTPURL(t *testing.T) {
	cfg := validProdConfigForValidate()
	cfg.Webhook.URL = "http://example.com/webhooks/authara"
	cfg.Webhook.Secret = strings.Repeat("s", 32)

	if err := cfg.validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestConfigValidate_ProdWebhookRejectsShortSecret(t *testing.T) {
	cfg := validProdConfigForValidate()
	cfg.Webhook.URL = "http://example.com/webhooks/authara"
	cfg.Webhook.Secret = "short-secret"

	if err := cfg.validate(); err == nil {
		t.Fatal("expected validate to reject short webhook secret in prod")
	}
}

func TestConfigValidate_ProdRejectsWeakInternalAPIToken(t *testing.T) {
	for _, token := range []string{"", "   ", strings.Repeat("i", 23), strings.Repeat("é", 12)} {
		cfg := validProdConfigForValidate()
		cfg.InternalAPI.Token = token

		err := cfg.validate()
		if err == nil || !strings.Contains(err.Error(), "AUTHARA_INTERNAL_API_TOKEN must be at least 24 characters") {
			t.Fatalf("validate error = %v, want internal API token length error", err)
		}
	}
}

func TestConfigValidate_ProdRejectsInternalAPITokenWithSurroundingWhitespace(t *testing.T) {
	cfg := validProdConfigForValidate()
	cfg.InternalAPI.Token = strings.Repeat("i", 24) + " "

	err := cfg.validate()
	if err == nil || !strings.Contains(err.Error(), "must not contain leading or trailing whitespace") {
		t.Fatalf("validate error = %v, want internal API token whitespace error", err)
	}
}

func TestConfigValidate_ProdAllowsMinimumInternalAPITokenLength(t *testing.T) {
	cfg := validProdConfigForValidate()
	cfg.InternalAPI.Token = strings.Repeat("i", 24)

	if err := cfg.validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestConfigValidate_DevAllowsEmptyInternalAPIToken(t *testing.T) {
	cfg := validProdConfigForValidate()
	cfg.Values.AppEnv = "dev"
	cfg.InternalAPI.Token = ""

	if err := cfg.validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestAdminValidateRejectsInvalidAuditRetention(t *testing.T) {
	cfg := validProdConfigForValidate()
	cfg.Admin.AuditRetentionDays = 0

	if err := cfg.Admin.validate(); err == nil {
		t.Fatal("expected admin audit retention validation error")
	}
}

func TestUIValidateRejectsUnsafeDefaultReturnTo(t *testing.T) {
	cfg := UI{AppName: DefaultAppName, DefaultReturnTo: "//evil.example"}

	if err := cfg.validate(); err == nil {
		t.Fatal("expected unsafe default return_to validation error")
	}
}

func TestUIValidateNormalizesAppName(t *testing.T) {
	cfg := UI{AppName: "  Example App  ", DefaultReturnTo: "/"}

	if err := cfg.validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.AppName != "Example App" {
		t.Fatalf("app name = %q, want %q", cfg.AppName, "Example App")
	}
}

func TestUIValidateRejectsInvalidAppName(t *testing.T) {
	for _, appName := range []string{"   ", strings.Repeat("a", maxAppNameRunes+1)} {
		cfg := UI{AppName: appName, DefaultReturnTo: "/"}
		if err := cfg.validate(); err == nil {
			t.Fatalf("expected app name %q to be rejected", appName)
		}
	}
}
