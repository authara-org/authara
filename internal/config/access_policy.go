package config

type AccessPolicy struct {
	AllowedEmailEnabled bool `env:"AUTHARA_ACCESS_POLICY_ALLOWLIST_ENABLED,default=false"`
}

func (a *AccessPolicy) validate() error {
	return nil
}
