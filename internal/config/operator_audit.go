package config

import "fmt"

type OperatorAudit struct {
	RetentionDays int `env:"AUTHARA_OPERATOR_AUDIT_RETENTION_DAYS,default=180"`
}

func (a *OperatorAudit) validate() error {
	if a.RetentionDays <= 0 {
		return fmt.Errorf("AUTHARA_OPERATOR_AUDIT_RETENTION_DAYS must be greater than 0")
	}
	return nil
}
