package redirect

import (
	"net/http"
	"net/url"
)

const ReturnToQueryParam = "return_to"

// QueryParam returns the raw return_to query parameter value.
func QueryParam(r *http.Request) string {
	return r.URL.Query().Get(ReturnToQueryParam)
}

// IsSafeReturnTo reports whether the provided value is a safe relative redirect path.
func IsSafeReturnTo(v string) bool {
	if v == "" {
		return false
	}

	// Must start with "/"
	if v[0] != '/' {
		return false
	}

	// Reject protocol-relative URLs like "//evil.com"
	if len(v) >= 2 && v[:2] == "//" {
		return false
	}

	return true
}

// NormalizeReturnTo validates the value and returns the normalized redirect target.
// If the value is invalid it returns ("", false).
func NormalizeReturnTo(v string) (string, bool) {
	if !IsSafeReturnTo(v) {
		return "", false
	}

	return v, true
}

// WithReturnTo builds a URL like:
//
//	/auth/login?return_to=%2Fauth%2Faccount
func WithReturnTo(path string, returnTo string) string {
	if returnTo == "" {
		return path
	}

	u, _ := url.Parse(path)

	q := u.Query()
	q.Set(ReturnToQueryParam, returnTo)
	u.RawQuery = q.Encode()

	return u.String()
}
