package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/google/uuid"
)

type emailVerificationStarter interface {
	BeginRequiredEmailVerification(context.Context, uuid.UUID, uuid.UUID, string, string, time.Time) (domain.EmailVerificationTransaction, error)
}

func RequireVerifiedEmailUI(
	service emailVerificationStarter,
	policy config.AuthenticationPolicyReader,
	audience token.Audience,
	now func() time.Time,
) func(http.Handler) http.Handler {
	return requireVerifiedEmail(service, policy, audience, now, true)
}

func RequireVerifiedEmailAPI(
	service emailVerificationStarter,
	policy config.AuthenticationPolicyReader,
	audience token.Audience,
	now func() time.Time,
) func(http.Handler) http.Handler {
	return requireVerifiedEmail(service, policy, audience, now, false)
}

func requireVerifiedEmail(
	service emailVerificationStarter,
	policy config.AuthenticationPolicyReader,
	audience token.Audience,
	now func() time.Time,
	ui bool,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !policy.CurrentAuthentication().EmailVerificationRequired {
				next.ServeHTTP(w, r)
				return
			}
			verified, _ := httpctx.EmailVerified(r.Context())
			if verified {
				next.ServeHTTP(w, r)
				return
			}

			userID, userOK := httpctx.UserID(r.Context())
			sessionID, sessionOK := httpctx.SessionID(r.Context())
			if !userOK || !sessionOK {
				emailVerificationFailure(w, r, ui)
				return
			}
			returnTo := r.URL.RequestURI()
			if returnTo == "" || returnTo[0] != '/' {
				returnTo = "/"
			}
			currentTime := now().UTC()
			transaction, err := service.BeginRequiredEmailVerification(
				r.Context(), userID, sessionID, string(audience), returnTo, currentTime,
			)
			if err != nil {
				emailVerificationFailure(w, r, ui)
				return
			}
			// The token can briefly carry stale verification state when another
			// concurrent request has just completed verification. In that case the
			// service returns no handoff transaction and the request may continue.
			if transaction.ID == uuid.Nil {
				next.ServeHTTP(w, r)
				return
			}
			session.ClearSessionCookies(w)
			session.SetEmailVerificationTransaction(w, transaction.ID, transaction.ExpiresAt, currentTime)
			w.Header().Set("Location", "/auth/verify-email")
			if ui {
				redirect.Redirect(w, r, "/auth/verify-email", http.StatusSeeOther)
				return
			}
			response.ErrorJSON(w, http.StatusForbidden, response.CodeForbidden, "Email verification required.")
		})
	}
}

func emailVerificationFailure(w http.ResponseWriter, r *http.Request, ui bool) {
	if ui {
		http.Error(w, "Could not start email verification", http.StatusServiceUnavailable)
		return
	}
	response.ErrorJSON(w, http.StatusServiceUnavailable, response.CodeInternalError, "Email verification unavailable.")
}
