package config

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/authara-org/authara/internal/http/kit/redirect"
)

const (
	DefaultAppName  = "Authara"
	maxAppNameRunes = 80
)

type UI struct {
	AppName         string `env:"AUTHARA_APP_NAME,default=Authara"`
	DefaultReturnTo string `env:"AUTHARA_DEFAULT_RETURN_TO,default=/"`
}

func (u *UI) validate() error {
	appName, err := normalizeAppName(u.AppName)
	if err != nil {
		return fmt.Errorf("invalid AUTHARA_APP_NAME: %w", err)
	}
	u.AppName = appName
	if _, ok := redirect.NormalizeReturnTo(u.DefaultReturnTo); !ok {
		return fmt.Errorf("invalid AUTHARA_DEFAULT_RETURN_TO %q: must be a safe relative path", u.DefaultReturnTo)
	}
	return nil
}

func normalizeAppName(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("must not be empty")
	}
	if utf8.RuneCountInString(value) > maxAppNameRunes {
		return "", fmt.Errorf("must be at most %d characters", maxAppNameRunes)
	}
	return value, nil
}
