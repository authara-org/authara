package config

import (
	"context"
	"testing"

	"github.com/sethvargo/go-envconfig"
)

func TestUIConfigDefaultsUserUIToEnabled(t *testing.T) {
	var cfg UI
	if err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target:   &cfg,
		Lookuper: envconfig.MapLookuper(nil),
	}); err != nil {
		t.Fatalf("process UI config: %v", err)
	}

	if !cfg.UserEnabled {
		t.Fatal("expected user-facing UI to be enabled by default")
	}
}

func TestUIConfigCanDisableUserUIFromEnvironment(t *testing.T) {
	var cfg UI
	if err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target: &cfg,
		Lookuper: envconfig.MapLookuper(map[string]string{
			"AUTHARA_USER_UI_ENABLED": "false",
		}),
	}); err != nil {
		t.Fatalf("process UI config: %v", err)
	}

	if cfg.UserEnabled {
		t.Fatal("expected user-facing UI to be disabled")
	}
}
