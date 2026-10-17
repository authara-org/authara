package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
)

func RequireAPIAccessAuth(sessionSvc *session.Service, audience token.Audience, now func() time.Time) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			accessToken, transport, ok := apiAccessToken(r)
			if !ok || accessToken == "" {
				response.ErrorJSON(
					w,
					http.StatusUnauthorized,
					response.CodeUnauthorized,
					"Unauthorized",
				)
				return
			}

			identity, err := sessionSvc.ValidateAccessToken(ctx, accessToken, audience, now())
			if err != nil {
				if errors.Is(err, token.ErrRevocationStoreUnavailable) {
					response.ErrorJSON(
						w,
						http.StatusServiceUnavailable,
						response.CodeInternalError,
						"Authentication service unavailable",
					)
					return
				}
				response.ErrorJSON(
					w,
					http.StatusUnauthorized,
					response.CodeUnauthorized,
					"Unauthorized",
				)
				return
			}

			ctx = httpctx.WithUserID(ctx, identity.UserID)
			ctx = httpctx.WithRoles(ctx, identity.Roles)
			ctx = httpctx.WithSessionID(ctx, identity.SessionID)
			ctx = httpctx.WithOrganizationID(ctx, identity.OrganizationID)
			ctx = httpctx.WithOrganizationRole(ctx, identity.OrganizationRole)
			ctx = httpctx.WithEmailVerified(ctx, identity.EmailVerified)
			ctx = httpctx.WithAuthTransport(ctx, transport)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func apiAccessToken(r *http.Request) (string, httpctx.AuthTransport, bool) {
	authorizationValues := r.Header.Values("Authorization")
	if len(authorizationValues) > 0 {
		if len(authorizationValues) != 1 {
			return "", "", false
		}
		authorization := strings.TrimSpace(authorizationValues[0])
		scheme, value, ok := strings.Cut(authorization, " ")
		value = strings.TrimSpace(value)
		if !ok || !strings.EqualFold(scheme, "Bearer") || value == "" {
			return "", "", false
		}
		return value, httpctx.AuthTransportBearer, true
	}

	accessToken, ok := session.ReadAccessToken(r)
	if !ok || accessToken == "" {
		return "", "", false
	}
	return accessToken, httpctx.AuthTransportCookie, true
}
