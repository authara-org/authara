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
