package config

import (
	"fmt"
	"time"
)

type Admin struct {
	AuditRetentionDays   int           `env:"AUTHARA_ADMIN_AUDIT_RETENTION_DAYS,default=180"`
	AuditCleanupInterval time.Duration `env:"AUTHARA_ADMIN_AUDIT_CLEANUP_INTERVAL,default=24h"`
}

func (a *Admin) validate() error {
	if a.AuditRetentionDays <= 0 {
		return fmt.Errorf("AUTHARA_ADMIN_AUDIT_RETENTION_DAYS must be greater than 0")
	}
	if a.AuditCleanupInterval <= 0 {
		return fmt.Errorf("AUTHARA_ADMIN_AUDIT_CLEANUP_INTERVAL must be greater than 0")
	}
	return nil
}
