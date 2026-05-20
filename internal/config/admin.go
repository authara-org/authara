package config

import "fmt"

type Admin struct {
	AuditRetentionDays int `env:"AUTHARA_ADMIN_AUDIT_RETENTION_DAYS,default=180"`
}

func (a *Admin) validate() error {
	if a.AuditRetentionDays <= 0 {
		return fmt.Errorf("AUTHARA_ADMIN_AUDIT_RETENTION_DAYS must be greater than 0")
	}
	return nil
}
