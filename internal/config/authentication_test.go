package config

import (
	"context"
	"testing"

	"github.com/sethvargo/go-envconfig"
)

func TestAuthenticationConfigEnablesUsernameLoginFromEnvironment(t *testing.T) {
	t.Setenv("AUTHARA_USERNAME_LOGIN_ENABLED", "true")

	var cfg Authentication
	if err := envconfig.Process(context.Background(), &cfg); err != nil {
		t.Fatalf("process authentication config: %v", err)
	}
	if !cfg.UsernameLoginEnabled {
		t.Fatal("expected username login to be enabled")
	}
}
