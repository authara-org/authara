package authflow

import (
	"net/http"
	"time"

	httpcontext "github.com/alexlup06/authgate/internal/http/context"
	"github.com/alexlup06/authgate/internal/http/redirect"
	"github.com/alexlup06/authgate/internal/session"
)

func TryRedirectAuthenticated(
	w http.ResponseWriter,
	r *http.Request,
	s *session.Service,
	accessTTL,
	refreshTTL time.Duration,
) bool {
	now := time.Now()

	if access, ok := session.ReadAccessToken(r); ok {
		if _, err := s.ValidateAccessToken(r.Context(), access, now); err == nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return true
		}
	}

	returnTo, ok := httpcontext.ReturnTo(r.Context())
	if !ok {
		returnTo = "/"
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
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return true
		}

		session.ClearSessionCookies(w)
	}

	return false
}
