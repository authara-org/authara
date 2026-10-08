package ui

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/flash"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/oauthstate"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/http/viewmodel"
	"github.com/authara-org/authara/internal/session"
	"github.com/google/uuid"
)

func (h *UIHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, ctx)
		return
	}

	idToken := r.FormValue("credential")
	nonce := r.FormValue("nonce")
	flow := r.FormValue("flow")
	linkID := r.FormValue("link_id")

	expectedNonce, ok := oauthstate.ReadNonce(r)
	if idToken == "" || nonce == "" {
		h.renderGoogleFlowError(w, r, ctx, flow)
		return
	}
	if !ok || subtle.ConstantTimeCompare([]byte(nonce), []byte(expectedNonce)) != 1 {
		h.recordGoogleAuthenticationDenial(ctx, r, flow, domain.SecurityEventReasonInvalidAssertion)
		h.renderGoogleFlowError(w, r, ctx, flow)
		return

	}

	identity, err := h.Google.VerifyIDToken(ctx, idToken, expectedNonce)
	if err != nil {
		h.recordGoogleAuthenticationDenial(ctx, r, flow, domain.SecurityEventReasonInvalidAssertion)
		h.renderGoogleFlowError(w, r, ctx, flow)
		return
	}

	if flow == string(viewmodel.AuthProviderFlowReauthenticate) {
		oauthstate.ClearNonce(w)
		userID, userOK := httpctx.UserID(ctx)
		sessionID, sessionOK := httpctx.SessionID(ctx)
		authenticationChallengeID, challengeOK := authenticationChallengeID(r)
		now := time.Now().UTC()
		if !userOK || !sessionOK || !challengeOK ||
			h.Session.ValidateAuthenticationChallenge(ctx, userID, sessionID, authenticationChallengeID, now) != nil {
			h.renderGoogleFlowError(w, r, ctx, flow)
			return
		}
		if err := h.Auth.VerifyExternalIdentity(ctx, userID, domain.ProviderGoogle, identity.OAuthID); err != nil {
			if errors.Is(err, auth.ErrInvalidCredentials) {
				if recordErr := h.Session.RecordReauthenticationDenied(ctx, userID, sessionID, domain.AuthenticationMethodGoogle, domain.SecurityEventReasonInvalidCredentials); recordErr != nil && h.Logger != nil {
					h.Logger.Error("record Google reauthentication denial", "err", recordErr)
				}
			}
			h.renderGoogleFlowError(w, r, ctx, flow)
			return
		}
		if h.Session.CompleteAuthenticationChallenge(ctx, userID, sessionID, authenticationChallengeID, domain.AuthenticationMethodGoogle, now) != nil {
			h.renderGoogleFlowError(w, r, ctx, flow)
			return
		}
		writeOAuthRedirect(w, httpctx.ReturnToOrManualDefault(ctx, "/auth/account"))
		return
	}

	if flow == string(viewmodel.AuthProviderFlowLink) {
		if !h.requireRecentProviderLinkAuthentication(w, r) {
			return
		}
		oauthstate.ClearNonce(w)
		if err := h.CompleteProviderLink(ctx, linkID, domain.ProviderGoogle, identity.OAuthID, identity.Email, identity.EmailVerified); err != nil {
			_ = flash.Set(w, flash.Message{
				Kind:    "error",
				Message: "Google sign-in failed. Please try again.",
			})
			redirect.Redirect(w, r, redirect.WithReturnTo("/auth/account", httpctx.ReturnToOrDefault(ctx)), http.StatusSeeOther)
			return
		}

		_ = flash.Set(w, flash.Message{
			Kind:    "success",
			Message: "Google account linked.",
		})

		writeOAuthRedirect(w, "/auth/account")
		return
	}

	if flow == string(viewmodel.AuthProviderFlowProof) {
		oauthstate.ClearNonce(w)
		parsedLinkID, err := uuid.Parse(strings.TrimSpace(linkID))
		if err != nil {
			h.renderError(w, r, ctx)
			return
		}
		link, err := h.Auth.GetPendingProviderLink(ctx, parsedLinkID)
		if err != nil {
			h.renderError(w, r, ctx)
			return
		}

		user, err := h.Auth.CompleteAccountRecoveryProviderLinkWithProviderProof(
			ctx,
			parsedLinkID,
			domain.ProviderGoogle,
			identity.OAuthID,
			time.Now().UTC(),
		)
		if err != nil {
			h.renderError(w, r, ctx)
			return
		}
		h.finishAccountRecoveryProviderProof(w, r, user, link.Provider, domain.AuthenticationMethodGoogle)
		return
	}

	oauthstate.ClearNonce(w)
	returnTo := httpctx.ReturnToOrDefault(ctx)
	if path, rawToken, ok := invitationAuthReturnTo(returnTo); ok {
		h.finishInvitationOAuth(w, r, path, rawToken, returnTo, identity.Email, identity.EmailVerified, identity.OAuthID)
		return
	}

	input := auth.LoginInput{
		Provider:              domain.ProviderGoogle,
		Email:                 identity.Email,
		OAuthID:               identity.OAuthID,
		ProviderEmailVerified: identity.EmailVerified,
	}

	user, err := h.Auth.Login(ctx, input)
	if err != nil {
		if errors.Is(err, auth.ErrAccountExistsMustLink) {
			link, linkErr := h.Auth.StartAccountRecoveryProviderLink(ctx, auth.OAuthIdentityInput{
				Provider:              domain.ProviderGoogle,
				Email:                 identity.Email,
				ProviderUserID:        identity.OAuthID,
				ProviderEmailVerified: identity.EmailVerified,
			}, time.Now().UTC())
			if linkErr != nil {
				h.renderError(w, r, ctx)
				return
			}

			u := url.URL{Path: "/auth/provider-links/confirm"}
			q := u.Query()
			q.Set("link_id", link.ID.String())
			if returnTo := httpctx.ReturnToOrDefault(ctx); returnTo != "" {
				q.Set("return_to", returnTo)
			}
			u.RawQuery = q.Encode()
			writeOAuthRedirect(w, u.String())
			return
		}
		h.renderError(w, r, ctx)
		return

	}

	audience := redirect.AudienceForPath(returnTo)
	ua := r.UserAgent()
	now := time.Now()
	accessToken, refreshToken, err := h.Session.CreateSession(ctx, user.ID, audience, domain.AuthenticationMethodGoogle, ua, now, httputil.ClientIPString(r))
	if err != nil {
		h.renderError(w, r, ctx)
		return

	}

	cookiePolicy := h.sessionCookiePolicy()
	session.SetAccessToken(w, accessToken, int(cookiePolicy.AccessTokenTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(cookiePolicy.RefreshTokenTTL.Seconds()))

	writeOAuthRedirect(w, returnTo)
}

func (h *UIHandler) recordGoogleAuthenticationDenial(ctx context.Context, r *http.Request, flow string, reasonCode string) {
	if flow == string(viewmodel.AuthProviderFlowLogin) {
		if err := h.Auth.RecordLoginDenied(ctx, domain.AuthenticationMethodGoogle, reasonCode); err != nil && h.Logger != nil {
			h.Logger.Error("record Google login denial", "err", err)
		}
		return
	}
	if flow != string(viewmodel.AuthProviderFlowReauthenticate) {
		return
	}
	userID, userOK := httpctx.UserID(ctx)
	sessionID, sessionOK := httpctx.SessionID(ctx)
	challengeID, challengeOK := authenticationChallengeID(r)
	if !userOK || !sessionOK || !challengeOK ||
		h.Session.ValidateAuthenticationChallenge(ctx, userID, sessionID, challengeID, time.Now().UTC()) != nil {
		return
	}
	if err := h.Session.RecordReauthenticationDenied(ctx, userID, sessionID, domain.AuthenticationMethodGoogle, reasonCode); err != nil && h.Logger != nil {
		h.Logger.Error("record Google reauthentication denial", "err", err)
	}
}

func (h *UIHandler) requireRecentProviderLinkAuthentication(w http.ResponseWriter, r *http.Request) bool {
	ctx := r.Context()
	userID, userOK := httpctx.UserID(ctx)
	sessionID, sessionOK := httpctx.SessionID(ctx)
	if !userOK || !sessionOK {
		response.ErrorJSON(w, http.StatusUnauthorized, response.CodeUnauthorized, "Unauthorized.")
		return false
	}
	if err := h.Session.RequireRecentAuthentication(ctx, userID, sessionID, time.Now().UTC()); err != nil {
		if errors.Is(err, session.ErrRecentAuthenticationRequired) {
			returnTo := httpctx.ReturnToOrManualDefault(ctx, "/auth/account")
			challenge, challengeErr := h.Session.StartAuthenticationChallenge(ctx, userID, sessionID, time.Now().UTC())
			if challengeErr != nil {
				response.ErrorJSON(w, http.StatusInternalServerError, response.CodeInternalError, "Session error.")
				return false
			}
			response.JSON(w, http.StatusPreconditionRequired, map[string]any{
				"error": map[string]string{
					"code":    string(response.CodeRecentAuthenticationRequired),
					"message": "Recent authentication is required.",
				},
				"authentication_challenge": map[string]any{
					"id":         challenge.ID,
					"expires_at": challenge.ExpiresAt,
				},
				"reauthenticate_url": redirect.WithReturnTo(
					"/auth/reauthenticate?authentication_challenge_id="+challenge.ID.String(),
					returnTo,
				),
			})
			return false
		}
		response.ErrorJSON(w, http.StatusInternalServerError, response.CodeInternalError, "Session error.")
		return false
	}
	return true
}

func writeOAuthRedirect(w http.ResponseWriter, location string) {
	w.Header().Set("X-Authara-Redirect", location)
	w.WriteHeader(http.StatusOK)
}

func isOAuthCallback(r *http.Request) bool {
	return r.URL.Path == "/auth/oauth/google/callback" || r.URL.Path == "/auth/oauth/apple/proof"
}

func (h *UIHandler) renderError(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	_ = flash.Set(w, flash.Message{
		Kind:    "error",
		Message: "Google sign-in failed. Please try again.",
	})
	redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", httpctx.ReturnToOrDefault(ctx)), http.StatusSeeOther)
}

func (h *UIHandler) renderGoogleFlowError(w http.ResponseWriter, r *http.Request, ctx context.Context, flow string) {
	if flow != string(viewmodel.AuthProviderFlowReauthenticate) {
		h.renderError(w, r, ctx)
		return
	}
	_ = flash.Set(w, flash.Message{
		Kind:    "error",
		Message: "Google authentication failed. Please try again.",
	})
	path := "/auth/reauthenticate"
	if challengeID, ok := authenticationChallengeID(r); ok {
		path += "?authentication_challenge_id=" + challengeID.String()
	}
	redirect.Redirect(w, r, redirect.WithReturnTo(path, httpctx.ReturnToOrDefault(ctx)), http.StatusSeeOther)
}
