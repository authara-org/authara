package config

import (
	"fmt"

	"github.com/authara-org/authara/internal/http/kit/redirect"
)

type UI struct {
	DefaultReturnTo string `env:"AUTHARA_DEFAULT_RETURN_TO,default=/"`
}

func (u *UI) validate() error {
	if _, ok := redirect.NormalizeReturnTo(u.DefaultReturnTo); !ok {
		return fmt.Errorf("invalid AUTHARA_DEFAULT_RETURN_TO %q: must be a safe relative path", u.DefaultReturnTo)
	}
	return nil
}
