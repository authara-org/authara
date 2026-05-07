package ui

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/flash"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	authview "github.com/authara-org/authara/internal/http/templates/auth"
	"github.com/authara-org/authara/internal/session"
	"github.com/google/uuid"
)

func (h *UIHandler) ProviderLinkConfirmPage(w http.ResponseWriter, r *http.Request) {
	linkIDStr := strings.TrimSpace(r.URL.Query().Get("link_id"))
	linkID, err := uuid.Parse(linkIDStr)
	if err != nil {
		h.redirectInvalidProviderLink(w, r)
		return
	}

	link, err := h.Auth.GetPendingProviderLink(r.Context(), linkID)
	if err != nil || link.ConsumedAt != nil || !link.ExpiresAt.After(time.Now().UTC()) {
		h.redirectInvalidProviderLink(w, r)
		return
	}

	email := ""
	if link.ProviderEmail != nil {
		email = *link.ProviderEmail
	}

	providers, err := h.Auth.ListUserAuthProviders(r.Context(), link.UserID)
	if err != nil {
		h.redirectInvalidProviderLink(w, r)
		return
	}

	_ = h.Render(
		w,
		r,
		http.StatusOK,
		authview.AccountCollision(linkIDStr, email, string(link.Provider), h.accountCollisionProofOptions(providers)),
	)
}

func (h *UIHandler) redirectInvalidProviderLink(w http.ResponseWriter, r *http.Request) {
	_ = flash.Set(w, flash.Message{
		Kind:    "error",
		Message: "This account connection request is invalid or expired. Please try again.",
	})
	redirect.Redirect(
		w,
		r,
		redirect.WithReturnTo("/auth/login", httpctx.ReturnToOrDefault(r.Context(), "/")),
		http.StatusSeeOther,
	)
}

func (h *UIHandler) ProviderLinkConfirmPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	linkIDStr := strings.TrimSpace(r.FormValue("link_id"))
	linkID, err := uuid.Parse(linkIDStr)
	if err != nil {
		h.renderFormError(w, r, http.StatusUnprocessableEntity, "Invalid or expired link.", authview.AccountCollisionForm(linkIDStr, "", "Google"))
		return
	}

	password := r.FormValue("password")
	user, err := h.Auth.CompleteAccountRecoveryProviderLinkWithPassword(ctx, linkID, password, time.Now().UTC())
	if err != nil {
		msg := "Could not connect Google. Please try again."
		if errors.Is(err, auth.ErrInvalidCredentials) {
			msg = "Invalid password."
		}
		if errors.Is(err, auth.ErrPendingProviderLinkExpired) {
			msg = "This connection request has expired. Please start again."
		}
		h.renderFormError(w, r, http.StatusUnprocessableEntity, msg, authview.AccountCollisionForm(linkIDStr, "", "Google"))
		return
	}

	returnTo := httpctx.ReturnToOrDefault(ctx, "/")
	audience := redirect.AudienceForPath(returnTo)
	now := time.Now()
	accessToken, refreshToken, err := h.Session.CreateSession(ctx, user.ID, audience, r.UserAgent(), now)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	session.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(h.RefreshTTL.Seconds()))

	if h.Logger != nil {
		h.Logger.Info("provider linked after account collision", "user_id", user.ID, "provider", "google")
	}

	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (h *UIHandler) accountCollisionProofOptions(providers []domain.AuthProvider) []authview.AccountCollisionProofOption {
	options := make([]authview.AccountCollisionProofOption, 0, len(providers))

	for _, provider := range providers {
		switch provider.Provider {
		case domain.ProviderPassword:
			options = append(options, authview.AccountCollisionProofOption{
				Provider: string(domain.ProviderPassword),
				Label:    "Password",
				Password: true,
			})
		case domain.ProviderGoogle:
			if h.Google == nil {
				continue
			}
			options = append(options, authview.AccountCollisionProofOption{
				Provider: string(domain.ProviderGoogle),
				Label:    "Google",
				ClientID: h.Google.ClientID,
			})
		}
	}

	return options
}
