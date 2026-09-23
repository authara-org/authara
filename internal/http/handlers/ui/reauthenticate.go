package ui

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	authview "github.com/authara-org/authara/internal/http/templates/auth"
	"github.com/authara-org/authara/internal/session"
	"github.com/google/uuid"
)

func (h *UIHandler) ReauthenticatePage(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpctx.UserID(r.Context())
	if !ok {
		h.renderUnauthorized(w, r)
		return
	}
	sessionID, sessionOK := httpctx.SessionID(r.Context())
	challengeID, challengeOK := authenticationChallengeID(r)
	if !sessionOK || !challengeOK {
		h.renderRequestError(w, r, http.StatusBadRequest, "Authentication challenge required.")
		return
	}
	if err := h.Session.ValidateAuthenticationChallenge(r.Context(), userID, sessionID, challengeID, time.Now().UTC()); err != nil {
		status := http.StatusInternalServerError
		message := "Could not load authentication challenge."
		if errors.Is(err, session.ErrAuthenticationChallengeInvalid) {
			status = http.StatusConflict
			message = "This authentication challenge has expired. Please try the action again."
		}
		h.renderRequestError(w, r, status, message)
		return
	}
	providers, err := h.Auth.ListUserAuthProviders(r.Context(), userID)
	if err != nil {
		h.renderRequestError(w, r, http.StatusInternalServerError, "Could not load authentication methods.")
		return
	}
	hasPassword := false
	hasGoogle := false
	for _, provider := range providers {
		switch provider.Provider {
		case domain.ProviderPassword:
			hasPassword = true
		case domain.ProviderGoogle:
			hasGoogle = true
		}
	}
	hasPasskey := false
	if h.Passkeys != nil {
		passkeys, err := h.Passkeys.ListUserPasskeys(r.Context(), userID)
		if err != nil {
			h.renderRequestError(w, r, http.StatusInternalServerError, "Could not load authentication methods.")
			return
		}
		hasPasskey = len(passkeys) > 0
	}
	googleClientID := ""
	if hasGoogle && h.Google != nil {
		googleClientID = h.Google.ClientID
	}
	_ = h.Render(w, r, http.StatusOK, authview.Reauthenticate(
		hasPassword,
		hasPasskey,
		googleClientID,
		challengeID.String(),
		r.URL.Query().Get("embedded") == "1",
	))
}

func (h *UIHandler) ReauthenticationCompletePage(w http.ResponseWriter, r *http.Request) {
	userID, userOK := httpctx.UserID(r.Context())
	sessionID, sessionOK := httpctx.SessionID(r.Context())
	challengeID, challengeOK := authenticationChallengeID(r)
	if !userOK || !sessionOK || !challengeOK {
		h.renderRequestError(w, r, http.StatusBadRequest, "Authentication challenge required.")
		return
	}
	if err := h.Session.ValidateCompletedAuthenticationChallenge(r.Context(), userID, sessionID, challengeID); err != nil {
		h.renderAuthenticationChallengeError(w, r, err)
		return
	}
	_ = h.Render(w, r, http.StatusOK, authview.ReauthenticationComplete(r.URL.Query().Get("embedded") == "1"))
}

func (h *UIHandler) ReauthenticatePasswordPost(w http.ResponseWriter, r *http.Request) {
	userID, userOK := httpctx.UserID(r.Context())
	sessionID, sessionOK := httpctx.SessionID(r.Context())
	if !userOK || !sessionOK {
		h.renderUnauthorized(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid form.")
		return
	}
	challengeID, err := uuid.Parse(strings.TrimSpace(r.FormValue("authentication_challenge_id")))
	if err != nil {
		h.renderRequestError(w, r, http.StatusBadRequest, "Authentication challenge required.")
		return
	}
	now := time.Now().UTC()
	if err := h.Session.ValidateAuthenticationChallenge(r.Context(), userID, sessionID, challengeID, now); err != nil {
		h.renderAuthenticationChallengeError(w, r, err)
		return
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowLoginAttempt(r.Context(), httputil.ClientIP(r), userID.String())
		if err != nil || !allowed {
			h.renderRequestError(w, r, http.StatusTooManyRequests, "Too many attempts. Please try again later.")
			return
		}
	}
	if err := h.Auth.VerifyPassword(r.Context(), userID, strings.TrimSpace(r.FormValue("password"))); err != nil {
		if !errors.Is(err, auth.ErrInvalidCredentials) && h.Logger != nil {
			h.Logger.Error("password reauthentication failed", "err", err)
		}
		h.renderRequestError(w, r, http.StatusUnprocessableEntity, "Password is incorrect.")
		return
	}
	if err := h.Session.CompleteAuthenticationChallenge(r.Context(), userID, sessionID, challengeID, domain.AuthenticationMethodPassword, now); err != nil {
		h.renderAuthenticationChallengeError(w, r, err)
		return
	}
	if r.FormValue("embedded") == "1" && r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Trigger", "autharaRecentAuthenticationComplete")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	redirect.Redirect(w, r, httpctx.ReturnToOrManualDefault(r.Context(), "/auth/account"), http.StatusSeeOther)
}

func authenticationChallengeID(r *http.Request) (uuid.UUID, bool) {
	challengeID, err := uuid.Parse(strings.TrimSpace(r.FormValue("authentication_challenge_id")))
	return challengeID, err == nil
}

func (h *UIHandler) renderAuthenticationChallengeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, session.ErrAuthenticationChallengeInvalid) {
		h.renderRequestError(w, r, http.StatusConflict, "This authentication challenge has expired. Please try the action again.")
		return
	}
	h.renderRequestError(w, r, http.StatusInternalServerError, "Could not update session.")
}
