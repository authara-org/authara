package redirect

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/authara-org/authara/internal/session/token"
)

func AudienceForPath(path string) token.Audience {
	if parsed, err := url.Parse(path); err == nil {
		path = parsed.Path
	}

	if pathIsWithin(path, "/admin") || pathIsWithin(path, "/auth/admin") {
		return token.AudienceAdmin
	}
	if pathIsWithin(path, "/operator") || pathIsWithin(path, "/auth/operator") {
		return token.AudienceOperator
	}
	return token.AudienceApp
}

func pathIsWithin(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func AudienceFromRequest(r *http.Request) (token.Audience, error) {
	raw := r.URL.Query().Get("audience")
	if raw == "" {
		return token.AudienceApp, nil
	}

	switch raw {
	case "app":
		return token.AudienceApp, nil
	case "admin":
		return token.AudienceAdmin, nil
	case "operator":
		return token.AudienceOperator, nil
	default:
		return "", errors.New("invalid audience")
	}
}
