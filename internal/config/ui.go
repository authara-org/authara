package config

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/authara-org/authara/internal/http/kit/redirect"
)

const (
	DefaultAppName  = "Authara"
	maxAppNameRunes = 80
)

type UI struct {
	UserEnabled     bool   `env:"AUTHARA_USER_UI_ENABLED,default=true"`
	AppName         string `env:"AUTHARA_APP_NAME,default=Authara"`
	DefaultReturnTo string `env:"AUTHARA_DEFAULT_RETURN_TO,default=/"`
	ErrorPages      ErrorPageURLs
}

type ErrorPageURLs struct {
	BadRequest          string `env:"AUTHARA_ERROR_PAGE_BAD_REQUEST_URL"`
	Unauthorized        string `env:"AUTHARA_ERROR_PAGE_UNAUTHORIZED_URL"`
	Forbidden           string `env:"AUTHARA_ERROR_PAGE_FORBIDDEN_URL"`
	NotFound            string `env:"AUTHARA_ERROR_PAGE_NOT_FOUND_URL"`
	Conflict            string `env:"AUTHARA_ERROR_PAGE_CONFLICT_URL"`
	Gone                string `env:"AUTHARA_ERROR_PAGE_GONE_URL"`
	UnprocessableEntity string `env:"AUTHARA_ERROR_PAGE_UNPROCESSABLE_ENTITY_URL"`
	TooManyRequests     string `env:"AUTHARA_ERROR_PAGE_TOO_MANY_REQUESTS_URL"`
	InternalServerError string `env:"AUTHARA_ERROR_PAGE_INTERNAL_SERVER_ERROR_URL"`
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
	if err := u.ErrorPages.validate(); err != nil {
		return err
	}
	return nil
}

func (u ErrorPageURLs) URL(status int) string {
	switch status {
	case 400:
		return u.BadRequest
	case 401:
		return u.Unauthorized
	case 403:
		return u.Forbidden
	case 404:
		return u.NotFound
	case 409:
		return u.Conflict
	case 410:
		return u.Gone
	case 422:
		return u.UnprocessableEntity
	case 429:
		return u.TooManyRequests
	case 500:
		return u.InternalServerError
	default:
		return ""
	}
}

func (u *ErrorPageURLs) validate() error {
	values := []struct {
		name  string
		value *string
	}{
		{"AUTHARA_ERROR_PAGE_BAD_REQUEST_URL", &u.BadRequest},
		{"AUTHARA_ERROR_PAGE_UNAUTHORIZED_URL", &u.Unauthorized},
		{"AUTHARA_ERROR_PAGE_FORBIDDEN_URL", &u.Forbidden},
		{"AUTHARA_ERROR_PAGE_NOT_FOUND_URL", &u.NotFound},
		{"AUTHARA_ERROR_PAGE_CONFLICT_URL", &u.Conflict},
		{"AUTHARA_ERROR_PAGE_GONE_URL", &u.Gone},
		{"AUTHARA_ERROR_PAGE_UNPROCESSABLE_ENTITY_URL", &u.UnprocessableEntity},
		{"AUTHARA_ERROR_PAGE_TOO_MANY_REQUESTS_URL", &u.TooManyRequests},
		{"AUTHARA_ERROR_PAGE_INTERNAL_SERVER_ERROR_URL", &u.InternalServerError},
	}
	for _, item := range values {
		value, err := normalizeOptionalHTTPURL(*item.value)
		if err != nil {
			return fmt.Errorf("invalid %s: %w", item.name, err)
		}
		*item.value = value
	}
	return nil
}

func normalizeOptionalHTTPURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	if parsed.Host == "" || (!strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https")) {
		return "", fmt.Errorf("must be an absolute HTTP or HTTPS URL")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("must not include credentials")
	}
	return value, nil
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
