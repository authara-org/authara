package config

import (
	"context"
	"testing"

	"github.com/sethvargo/go-envconfig"
)

func TestOperatorAuditConfigDefaultsRetentionTo180Days(t *testing.T) {
	var cfg OperatorAudit
	if err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target:   &cfg,
		Lookuper: envconfig.MapLookuper(nil),
	}); err != nil {
		t.Fatalf("process operator audit config: %v", err)
	}

	if cfg.RetentionDays != 180 {
		t.Fatalf("operator audit retention = %d days, want 180", cfg.RetentionDays)
	}
}

func TestOperatorAuditConfigReadsRetentionFromEnvironment(t *testing.T) {
	var cfg OperatorAudit
	if err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target: &cfg,
		Lookuper: envconfig.MapLookuper(map[string]string{
			"AUTHARA_OPERATOR_AUDIT_RETENTION_DAYS": "30",
		}),
	}); err != nil {
		t.Fatalf("process operator audit config: %v", err)
	}

	if cfg.RetentionDays != 30 {
		t.Fatalf("operator audit retention = %d days, want 30", cfg.RetentionDays)
	}
}

func TestOperatorAuditConfigRejectsInvalidRetention(t *testing.T) {
	for _, days := range []int{0, -1} {
		cfg := OperatorAudit{RetentionDays: days}
		if err := cfg.validate(); err == nil {
			t.Fatalf("retention of %d days unexpectedly passed validation", days)
		}
	}
}
