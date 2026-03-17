package config

import (
	"fmt"
	"net/url"
	"strings"
)

type Values struct {
	AppEnv    string `env:"APP_ENV,default=dev"`
	PublicURL string `env:"PUBLIC_URL,required"`

	HttpAddr string
}

func (v *Values) validate() error {
	switch v.AppEnv {
	case "dev", "prod":
		// ok
	default:
		return fmt.Errorf("invalid APP_ENV %q (allowed: dev, prod)", v.AppEnv)
	}

	u, err := url.Parse(v.PublicURL)
	if err != nil {
		return fmt.Errorf("invalid PUBLIC_URL: %w", err)
	}

	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid PUBLIC_URL %q: must include scheme and host", v.PublicURL)
	}

	if u.Path != "" && u.Path != "/" {
		return fmt.Errorf("invalid PUBLIC_URL %q: must not include a path", v.PublicURL)
	}

	if strings.HasSuffix(v.PublicURL, "/") {
		return fmt.Errorf("invalid PUBLIC_URL %q: must not have trailing slash", v.PublicURL)
	}
	return nil
}
