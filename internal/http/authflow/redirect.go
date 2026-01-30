package authflow

import (
	"net/http"
	"strings"
	"time"

	httpcontext "github.com/alexlup06-authgate/authgate/internal/http/context"
	"github.com/alexlup06-authgate/authgate/internal/http/redirect"
	"github.com/alexlup06-authgate/authgate/internal/session"
)

func TryRedirectAuthenticated(
	w http.ResponseWriter,
	r *http.Request,
	s *session.Service,
	accessTTL,
	refreshTTL time.Duration,
) bool {
	now := time.Now()

	returnTo, ok := httpcontext.ReturnTo(r.Context())
	if !ok {
		current := r.URL.Path
		if r.URL.RawQuery != "" {
			current += "?" + r.URL.RawQuery
		}

		if !strings.HasPrefix(current, "/auth/") {
			returnTo = current
		} else {
			returnTo = "/"
		}
	}

	audience := redirect.AudienceForPath(returnTo)

	if refresh, ok := session.ReadRefreshToken(r); ok {
		accessToken, newRefreshToken, err := s.RefreshSession(
			r.Context(),
			refresh,
			audience,
			now,
		)
		if err == nil {
			session.SetAccessToken(w, accessToken, int(accessTTL.Seconds()))
			session.SetRefreshToken(w, newRefreshToken, int(refreshTTL.Seconds()))
			redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
			return true
		}

		session.ClearSessionCookies(w)
	}

	return false
}
