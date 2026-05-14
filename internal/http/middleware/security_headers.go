package middleware

import (
	"net/http"
	"strings"
)

const (
	headerContentSecurityPolicy = "Content-Security-Policy"
	headerFrameOptions          = "X-Frame-Options"
	headerContentTypeOptions    = "X-Content-Type-Options"
	headerReferrerPolicy        = "Referrer-Policy"
)

type SecurityHeadersConfig struct {
	AllowGoogleOAuth bool
}

func SecurityHeaders(cfg SecurityHeadersConfig) func(http.Handler) http.Handler {
	csp := buildContentSecurityPolicy(cfg)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(headerContentSecurityPolicy, csp)
			w.Header().Set(headerFrameOptions, "DENY")
			w.Header().Set(headerContentTypeOptions, "nosniff")
			w.Header().Set(headerReferrerPolicy, "same-origin")

			next.ServeHTTP(w, r)
		})
	}
}

func buildContentSecurityPolicy(cfg SecurityHeadersConfig) string {
	scriptSrc := []string{"'self'", "'unsafe-inline'", "'unsafe-eval'"}
	imgSrc := []string{"'self'", "data:"}
	connectSrc := []string{"'self'"}
	frameSrc := []string{"'none'"}

	if cfg.AllowGoogleOAuth {
		scriptSrc = append(scriptSrc, "https://accounts.google.com")
		imgSrc = append(imgSrc, "https://www.gstatic.com", "https://ssl.gstatic.com")
		connectSrc = append(connectSrc, "https://accounts.google.com")
		frameSrc = []string{"https://accounts.google.com"}
	}

	directives := []string{
		"default-src 'self'",
		"base-uri 'self'",
		"object-src 'none'",
		"frame-ancestors 'none'",
		"form-action 'self'",
		"font-src 'self'",
		"img-src " + strings.Join(imgSrc, " "),
		"connect-src " + strings.Join(connectSrc, " "),
		"frame-src " + strings.Join(frameSrc, " "),
		"script-src " + strings.Join(scriptSrc, " "),
		"style-src 'self' 'unsafe-inline'",
	}

	return strings.Join(directives, "; ")
}
