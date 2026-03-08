package redirect

import (
	"errors"
	"net/http"
	"strings"

	"github.com/authara-org/authara/internal/session/token"
)

func AudienceForPath(path string) token.Audience {
	if strings.HasPrefix(path, "/admin") {
		return token.AudienceAdmin
	}
	return token.AudienceApp
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
	default:
		return "", errors.New("invalid audience")
	}
}
