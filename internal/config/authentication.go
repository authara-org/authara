package config

type Authentication struct {
	UsernameLoginEnabled bool `env:"AUTHARA_USERNAME_LOGIN_ENABLED,default=false"`
}
