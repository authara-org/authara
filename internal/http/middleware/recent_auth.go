package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/session"
	"github.com/google/uuid"
)

func RequireRecentAuthenticationAPI(sessionService *session.Service, now func() time.Time) func(http.Handler) http.Handler {
	return requireRecentAuthentication(sessionService, now, true)
}

func RequireRecentAuthenticationUI(sessionService *session.Service, now func() time.Time) func(http.Handler) http.Handler {
	return requireRecentAuthentication(sessionService, now, false)
}

func requireRecentAuthentication(sessionService *session.Service, now func() time.Time, api bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, userOK := httpctx.UserID(r.Context())
			sessionID, sessionOK := httpctx.SessionID(r.Context())
			if !userOK || !sessionOK {
				http.Error(w, "Unauthorized.", http.StatusUnauthorized)
				return
			}
			err := sessionService.RequireRecentAuthentication(r.Context(), userID, sessionID, now().UTC())
			if err == nil {
				next.ServeHTTP(w, r)
				return
			}
			if !errors.Is(err, session.ErrRecentAuthenticationRequired) {
				if api {
					response.ErrorJSON(w, http.StatusInternalServerError, response.CodeInternalError, "Session error.")
				} else {
					http.Error(w, "Session error.", http.StatusInternalServerError)
				}
				return
			}

			challenge, err := sessionService.StartAuthenticationChallenge(r.Context(), userID, sessionID, now().UTC())
			if err != nil {
				if api {
					response.ErrorJSON(w, http.StatusInternalServerError, response.CodeInternalError, "Session error.")
				} else {
					http.Error(w, "Session error.", http.StatusInternalServerError)
				}
				return
			}

			location := recentAuthenticationURL(r, challenge.ID)
			if api {
				writeRecentAuthenticationRequired(w, location, challenge)
				return
			}
			if r.Header.Get("HX-Request") == "true" {
				writeRecentAuthenticationRequired(w, location, challenge)
				return
			}
			if strings.Contains(r.Header.Get("Accept"), "application/json") || strings.Contains(r.Header.Get("Content-Type"), "application/json") {
				writeRecentAuthenticationRequired(w, location, challenge)
				return
			}
			http.Redirect(w, r, location, http.StatusSeeOther)
		})
	}
}

func writeRecentAuthenticationRequired(w http.ResponseWriter, reauthenticateURL string, challenge domain.AuthenticationChallenge) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusPreconditionRequired)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    string(response.CodeRecentAuthenticationRequired),
			"message": "Recent authentication is required.",
		},
		"authentication_challenge": map[string]any{
			"id":         challenge.ID,
			"expires_at": challenge.ExpiresAt,
		},
		"reauthenticate_url": reauthenticateURL,
	})
}

func recentAuthenticationURL(r *http.Request, challengeID uuid.UUID) string {
	returnTo := "/auth/account"
	if raw := r.Referer(); raw != "" {
		if parsed, err := url.Parse(raw); err == nil && parsed.Host == r.Host && strings.HasPrefix(parsed.Path, "/") {
			returnTo = parsed.RequestURI()
		}
	}
	query := url.Values{}
	query.Set("authentication_challenge_id", challengeID.String())
	query.Set("return_to", returnTo)
	return "/auth/reauthenticate?" + query.Encode()
}
