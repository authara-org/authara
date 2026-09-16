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
		Session: Session{
			RefreshTokenTTL: time.Hour,
		},
		Admin: Admin{
			AuditRetentionDays: 180,
		},
		Email: Email{
			Provider: "smtp",
			SMTPTLS:  true,
		},
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

func TestAdminValidateRejectsInvalidAuditRetention(t *testing.T) {
	cfg := validProdConfigForValidate()
	cfg.Admin.AuditRetentionDays = 0

	if err := cfg.Admin.validate(); err == nil {
		t.Fatal("expected admin audit retention validation error")
	}
}

func TestUIValidateRejectsUnsafeDefaultReturnTo(t *testing.T) {
	cfg := UI{DefaultReturnTo: "//evil.example"}

	if err := cfg.validate(); err == nil {
		t.Fatal("expected unsafe default return_to validation error")
	}
}
