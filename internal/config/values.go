package config

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

type Values struct {
	AppEnv               string `env:"APP_ENV,default=dev"`
	PublicURL            string `env:"PUBLIC_URL,required"`
	CookieDomain         string `env:"AUTHARA_COOKIE_DOMAIN"`
	TrustProxyHeaders    bool   `env:"AUTHARA_TRUST_PROXY_HEADERS,default=false"`
	TrustedProxyCIDRsRaw string `env:"AUTHARA_TRUSTED_PROXY_CIDRS"`

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

	if err := v.validateCookieDomain(u); err != nil {
		return err
	}
	return nil
}

func (v *Values) validateCookieDomain(publicURL *url.URL) error {
	if v.CookieDomain == "" {
		return nil
	}
	if strings.TrimSpace(v.CookieDomain) != v.CookieDomain {
		return fmt.Errorf("invalid AUTHARA_COOKIE_DOMAIN %q: must not contain leading or trailing whitespace", v.CookieDomain)
	}

	domain := strings.ToLower(strings.TrimPrefix(v.CookieDomain, "."))
	if domain == "" || strings.HasSuffix(domain, ".") {
		return fmt.Errorf("invalid AUTHARA_COOKIE_DOMAIN %q", v.CookieDomain)
	}
	if err := (&http.Cookie{Name: "authara_cookie_domain_check", Value: "1", Domain: domain}).Valid(); err != nil {
		return fmt.Errorf("invalid AUTHARA_COOKIE_DOMAIN %q: %w", v.CookieDomain, err)
	}
	if _, err := publicsuffix.EffectiveTLDPlusOne(domain); err != nil {
		return fmt.Errorf("invalid AUTHARA_COOKIE_DOMAIN %q: must be a registrable domain", v.CookieDomain)
	}

	host := strings.ToLower(publicURL.Hostname())
	if host != domain && !strings.HasSuffix(host, "."+domain) {
		return fmt.Errorf(
			"invalid AUTHARA_COOKIE_DOMAIN %q: must match PUBLIC_URL host %q or be its parent domain",
			v.CookieDomain,
			host,
		)
	}

	v.CookieDomain = domain
	return nil
}
