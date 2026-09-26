package config

import (
	"context"
	"testing"

	"github.com/sethvargo/go-envconfig"
)

func TestAuthenticationConfigEnablesUsernameLoginFromEnvironment(t *testing.T) {
	t.Setenv("AUTHARA_USERNAME_LOGIN_ENABLED", "true")
	t.Setenv("AUTHARA_PASSWORD_MIN_LENGTH", "15")

	var cfg Authentication
	if err := envconfig.Process(context.Background(), &cfg); err != nil {
		t.Fatalf("process authentication config: %v", err)
	}
	if !cfg.UsernameLoginEnabled {
		t.Fatal("expected username login to be enabled")
	}
	if cfg.PasswordMinimumLength != 15 {
		t.Fatalf("unexpected password configuration: %+v", cfg)
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate authentication config: %v", err)
	}
}

func TestAuthenticationConfigRejectsInvalidPasswordPolicy(t *testing.T) {
	for _, cfg := range []Authentication{
		{PasswordMinimumLength: 7},
		{PasswordMinimumLength: 129},
	} {
		if err := cfg.validate(); err == nil {
			t.Fatalf("expected invalid password policy to fail: %+v", cfg)
		}
	}
}
