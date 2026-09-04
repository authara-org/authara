package config

import (
	"context"
	"testing"

	"github.com/sethvargo/go-envconfig"
)

func TestObservabilityConfigDefaultsMetricsToDisabled(t *testing.T) {
	var cfg Observability
	if err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target:   &cfg,
		Lookuper: envconfig.MapLookuper(nil),
	}); err != nil {
		t.Fatalf("process observability config: %v", err)
	}

	if cfg.Enabled {
		t.Fatal("expected metrics to be disabled by default")
	}
}

func TestObservabilityConfigEnablesMetricsFromEnvironment(t *testing.T) {
	var cfg Observability
	if err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target: &cfg,
		Lookuper: envconfig.MapLookuper(map[string]string{
			"AUTHARA_METRICS_ENABLED": "true",
		}),
	}); err != nil {
		t.Fatalf("process observability config: %v", err)
	}

	if !cfg.Enabled {
		t.Fatal("expected metrics to be enabled")
	}
}
