package config

type AccessPolicy struct {
	AllowedEmailEnabled bool `env:"AUTHARA_ALLOWED_EMAIL_LIST_ENABLED,default=false"`
}

func (a *AccessPolicy) validate() error {
	return nil
}
