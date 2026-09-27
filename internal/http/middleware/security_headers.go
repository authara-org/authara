package middleware

import (
	"net/http"
	"strings"
)

const (
	headerCacheControl          = "Cache-Control"
	headerContentSecurityPolicy = "Content-Security-Policy"
	headerFrameOptions          = "X-Frame-Options"
	headerContentTypeOptions    = "X-Content-Type-Options"
	headerReferrerPolicy        = "Referrer-Policy"
)

type SecurityHeadersConfig struct {
	AllowGoogleOAuth bool
	AllowShowcase    bool
}

func SecurityHeaders(cfg SecurityHeadersConfig) func(http.Handler) http.Handler {
	csp := buildContentSecurityPolicy(cfg)
	reauthenticationCSP := strings.Replace(csp, "frame-ancestors 'none'", "frame-ancestors 'self'", 1)
	showcaseCSP := strings.Replace(reauthenticationCSP, "form-action 'self'", "form-action 'none'", 1)
	referrerPolicy := "same-origin"
	if cfg.AllowGoogleOAuth {
		referrerPolicy = "strict-origin-when-cross-origin"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(headerCacheControl, "no-store")
			if isShowcaseFrame(r, cfg.AllowShowcase) {
				w.Header().Set(headerContentSecurityPolicy, showcaseCSP)
				w.Header().Set(headerFrameOptions, "SAMEORIGIN")
			} else if isSameOriginReauthenticationFrame(r) {
				w.Header().Set(headerContentSecurityPolicy, reauthenticationCSP)
				w.Header().Set(headerFrameOptions, "SAMEORIGIN")
			} else {
				w.Header().Set(headerContentSecurityPolicy, csp)
				w.Header().Set(headerFrameOptions, "DENY")
			}
			w.Header().Set(headerContentTypeOptions, "nosniff")
			w.Header().Set(headerReferrerPolicy, referrerPolicy)

			next.ServeHTTP(w, r)
		})
	}
}

func isSameOriginReauthenticationFrame(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	return r.URL.Path == "/auth/reauthenticate" || r.URL.Path == "/auth/reauthenticate/complete"
}

func isShowcaseFrame(r *http.Request, allowShowcase bool) bool {
	return allowShowcase && r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/auth/showcase/pages/")
}

func buildContentSecurityPolicy(cfg SecurityHeadersConfig) string {
	scriptSrc := []string{"'self'", "'unsafe-inline'", "'unsafe-eval'"}
	imgSrc := []string{"'self'", "data:"}
	connectSrc := []string{"'self'"}
	frameSrc := []string{"'self'"}
	styleSrc := []string{"'self'", "'unsafe-inline'"}

	if cfg.AllowGoogleOAuth {
		scriptSrc = append(scriptSrc, "https://accounts.google.com")
		imgSrc = append(imgSrc, "https://www.gstatic.com", "https://ssl.gstatic.com")
		connectSrc = append(connectSrc, "https://accounts.google.com")
		frameSrc = append(frameSrc, "https://accounts.google.com")
		styleSrc = append(styleSrc, "https://accounts.google.com")
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
		"style-src " + strings.Join(styleSrc, " "),
	}

	return strings.Join(directives, "; ")
}
