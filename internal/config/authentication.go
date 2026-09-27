package config

import "fmt"

const (
	PasswordMinimumLengthFloor = 8
	PasswordMaximumLength      = 128

	PasskeyCloneResponseAlert             = "alert"
	PasskeyCloneResponseRestrict          = "restrict"
	PasskeyCloneResponseRestrictAndRevoke = "restrict_and_revoke"
)

type Authentication struct {
	UsernameLoginEnabled   bool   `env:"AUTHARA_USERNAME_LOGIN_ENABLED,default=false"`
	PasswordMinimumLength  int    `env:"AUTHARA_PASSWORD_MIN_LENGTH,default=8"`
	PasskeyCloneResponse   string `env:"AUTHARA_PASSKEY_CLONE_RESPONSE,default=alert"`
	PasskeyCloneNotifyUser bool   `env:"AUTHARA_PASSKEY_CLONE_NOTIFY_USER,default=true"`
}

func (a *Authentication) validate() error {
	if a.PasswordMinimumLength < PasswordMinimumLengthFloor || a.PasswordMinimumLength > PasswordMaximumLength {
		return fmt.Errorf(
			"AUTHARA_PASSWORD_MIN_LENGTH must be between %d and %d",
			PasswordMinimumLengthFloor,
			PasswordMaximumLength,
		)
	}
	switch a.PasskeyCloneResponse {
	case PasskeyCloneResponseAlert, PasskeyCloneResponseRestrict, PasskeyCloneResponseRestrictAndRevoke:
	default:
		return fmt.Errorf(
			"AUTHARA_PASSKEY_CLONE_RESPONSE must be one of %s, %s, %s",
			PasskeyCloneResponseAlert,
			PasskeyCloneResponseRestrict,
			PasskeyCloneResponseRestrictAndRevoke,
		)
	}

	return nil
}
