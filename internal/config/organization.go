package config

import (
	"fmt"
	"time"
)

const (
	OrgModePersonal = "personal"
	OrgModeSingle   = "single"
	OrgModeMulti    = "multi"
)

type Organization struct {
	Mode                                string `env:"AUTHARA_ORG_MODE,default=single"`
	PublicOrganizationManagementEnabled bool   `env:"AUTHARA_PUBLIC_ORGANIZATION_MANAGEMENT_ENABLED,default=false"`
	InvitationTTLRaw                    string `env:"AUTHARA_ORGANIZATION_INVITATION_TTL,default=168h"`

	InvitationTTL time.Duration
}

func (o *Organization) validate() error {
	switch o.Mode {
	case OrgModePersonal, OrgModeSingle, OrgModeMulti:
		return nil
	default:
		return fmt.Errorf("invalid AUTHARA_ORG_MODE %q (allowed: personal, single, multi)", o.Mode)
	}
}

func (o *Organization) parse() error {
	ttl, err := time.ParseDuration(o.InvitationTTLRaw)
	if err != nil {
		return fmt.Errorf("invalid AUTHARA_ORGANIZATION_INVITATION_TTL: %q", o.InvitationTTLRaw)
	}
	if ttl <= 0 {
		return fmt.Errorf("AUTHARA_ORGANIZATION_INVITATION_TTL must be greater than 0")
	}
	o.InvitationTTL = ttl
	return nil
}
