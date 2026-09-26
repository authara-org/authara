package config

import "fmt"

const (
	PasswordMinimumLengthFloor = 8
	PasswordMaximumLength      = 128
)

type Authentication struct {
	UsernameLoginEnabled  bool `env:"AUTHARA_USERNAME_LOGIN_ENABLED,default=false"`
	PasswordMinimumLength int  `env:"AUTHARA_PASSWORD_MIN_LENGTH,default=8"`
}

func (a *Authentication) validate() error {
	if a.PasswordMinimumLength < PasswordMinimumLengthFloor || a.PasswordMinimumLength > PasswordMaximumLength {
		return fmt.Errorf(
			"AUTHARA_PASSWORD_MIN_LENGTH must be between %d and %d",
			PasswordMinimumLengthFloor,
			PasswordMaximumLength,
		)
	}

	return nil
}
