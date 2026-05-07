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
	"github.com/authara-org/authara/internal/http/kit/oauthstate"
	"github.com/authara-org/authara/internal/http/kit/redirect"
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
	if idToken == "" || nonce == "" || !ok ||
		subtle.ConstantTimeCompare([]byte(nonce), []byte(expectedNonce)) != 1 {
		h.renderError(w, r, ctx)
		return

	}

	identity, err := h.Google.VerifyIDToken(ctx, idToken, expectedNonce)
	if err != nil {
		h.renderError(w, r, ctx)
		return
	}
	oauthstate.ClearNonce(w)

	if flow == string(viewmodel.AuthProviderFlowLink) {
		if err := h.CompleteProviderLink(ctx, linkID, domain.ProviderGoogle, identity.OAuthID, identity.Email, identity.EmailVerified); err != nil {
			_ = flash.Set(w, flash.Message{
				Kind:    "error",
				Message: "Google login failed. Please try again.",
			})
			redirect.Redirect(w, r, redirect.WithReturnTo("/auth/account", httpctx.ReturnToOrDefault(ctx, "/")), http.StatusSeeOther)
			return
		}

		_ = flash.Set(w, flash.Message{
			Kind:    "success",
			Message: "Google account linked.",
		})

		redirect.Redirect(w, r, "/auth/account", http.StatusSeeOther)
		return
	}

	if flow == string(viewmodel.AuthProviderFlowProof) {
		parsedLinkID, err := uuid.Parse(strings.TrimSpace(linkID))
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

		returnTo := httpctx.ReturnToOrDefault(ctx, "/")
		audience := redirect.AudienceForPath(returnTo)
		now := time.Now()
		accessToken, refreshToken, err := h.Session.CreateSession(ctx, user.ID, audience, r.UserAgent(), now)
		if err != nil {
			h.renderError(w, r, ctx)
			return
		}

		session.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
		session.SetRefreshToken(w, refreshToken, int(h.RefreshTTL.Seconds()))

		if h.Logger != nil {
			h.Logger.Info("provider linked after account collision", "user_id", user.ID, "provider", domain.ProviderGoogle)
		}
		_ = flash.Set(w, flash.Message{
			Kind:    "success",
			Message: "Sign-in provider was connected to your account.",
		})

		w.Header().Set("X-Authara-Redirect", returnTo)
		w.WriteHeader(http.StatusOK)
		return
	}

	input := auth.LoginInput{
		Provider: domain.ProviderGoogle,
		Email:    identity.Email,
		OAuthID:  identity.OAuthID,
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
			if returnTo := httpctx.ReturnToOrDefault(ctx, "/"); returnTo != "" {
				q.Set("return_to", returnTo)
			}
			u.RawQuery = q.Encode()
			w.Header().Set("X-Authara-Redirect", u.String())
			w.WriteHeader(http.StatusOK)
			return
		}
		h.renderError(w, r, ctx)
		return

	}

	returnTo, ok := httpctx.ReturnTo(r.Context())
	if !ok {
		returnTo = "/"
	}

	audience := redirect.AudienceForPath(returnTo)
	ua := r.UserAgent()
	now := time.Now()
	accessToken, refreshToken, err := h.Session.CreateSession(ctx, user.ID, audience, ua, now)
	if err != nil {
		h.renderError(w, r, ctx)
		return

	}

	session.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(h.RefreshTTL.Seconds()))

	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (h *UIHandler) renderError(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	_ = flash.Set(w, flash.Message{
		Kind:    "error",
		Message: "Google login failed. Please try again.",
	})
	redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", httpctx.ReturnToOrDefault(ctx, "/")), http.StatusSeeOther)
}
