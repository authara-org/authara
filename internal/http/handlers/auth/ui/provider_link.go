package ui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	authhandler "github.com/authara-org/authara/internal/http/handlers/auth"
	"github.com/authara-org/authara/internal/http/kit/htmx"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
	userview "github.com/authara-org/authara/internal/http/templates/user"
	"github.com/authara-org/authara/internal/http/viewmodel"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *UIHandler) ProviderLinkStartPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sessionID, ok := httpctx.SessionID(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	provider, err := parseLinkProvider(chi.URLParam(r, "provider"))
	if err != nil {
		http.Error(w, "invalid provider", http.StatusBadRequest)
		return
	}

	linkID, err := h.Auth.StartProviderLink(
		ctx,
		userID,
		sessionID,
		provider,
		time.Now().UTC(),
	)
	if err != nil {
		http.Error(w, "could not start provider link", http.StatusUnprocessableEntity)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"link_id": linkID.String(),
	})
}

func (h *UIHandler) CompleteProviderLink(
	ctx context.Context,
	linkIDStr string,
	provider domain.Provider,
	providerUserID string,
	providerEmail string,
	providerEmailVerified bool,
) error {
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return auth.ErrPendingProviderLinkInvalid
	}

	sessionID, ok := httpctx.SessionID(ctx)
	if !ok {
		return auth.ErrPendingProviderLinkInvalid
	}

	linkID, err := uuid.Parse(strings.TrimSpace(linkIDStr))
	if err != nil {
		return auth.ErrPendingProviderLinkInvalid
	}

	return h.Auth.CompleteProviderLink(
		ctx,
		linkID,
		userID,
		sessionID,
		provider,
		providerUserID,
		providerEmail,
		providerEmailVerified,
		time.Now().UTC(),
	)
}

func (h *UIHandler) GoogleLinkStartPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sessionID, ok := httpctx.SessionID(ctx)
	if !ok {
		http.Error(w, "missing session", http.StatusUnauthorized)
		return
	}

	linkID, err := h.Auth.StartProviderLink(
		ctx,
		userID,
		sessionID,
		domain.ProviderGoogle,
		time.Now().UTC(),
	)
	if err != nil {
		http.Error(w, "could not start link", http.StatusUnprocessableEntity)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"link_id": linkID.String(),
	})
}

func (h *UIHandler) UnlinkProviderPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	providerStr := chi.URLParam(r, "provider")
	provider, err := parseProvider(providerStr)
	if err != nil {
		http.Error(w, "invalid provider", http.StatusBadRequest)
		return
	}

	err = h.Auth.UnlinkAuthProvider(ctx, userID, provider)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrCannotRemoveLastAuthProvider):
			htmx.ReSwap(w, "none")
			_ = h.Render(w, r, http.StatusUnprocessableEntity,
				toast.ToastMessage(toast.Error, "You need at least one sign-in method."),
			)
			return
		default:
			htmx.ReSwap(w, "none")
			_ = h.Render(w, r, http.StatusInternalServerError,
				toast.ToastMessage(toast.Error, "Could not unlink provider."),
			)
			return
		}
	}

	providers, err := h.Auth.ListUserAuthProviders(ctx, userID)
	if err != nil {
		htmx.ReSwap(w, "none")
		_ = h.Render(
			w,
			r,
			http.StatusInternalServerError,
			toast.ToastMessage(toast.Error, "Could not load sign-in methods."),
		)
		return
	}

	vm := viewmodel.AuthProvidersFromDomain(providers, h.OAuthProviders.Providers)

	_ = h.Render(
		w,
		r,
		http.StatusOK,
		templ.Join(
			toast.ToastMessage(toast.Success, "Successfully removed Sign-In method."),
			userview.LinkedProvidersSection(vm, h.Google.ClientID),
		),
	)
}

func (h *UIHandler) PasswordLinkPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		htmx.ReSwap(w, "none")
		_ = h.Render(w, r, http.StatusUnauthorized, toast.ToastMessage(toast.Error, "Unauthorized."))
		return
	}

	if err := r.ParseForm(); err != nil {
		htmx.ReSwap(w, "none")
		_ = h.Render(w, r, http.StatusBadRequest, toast.ToastMessage(toast.Error, "Invalid form."))
		return
	}

	password := strings.TrimSpace(r.FormValue("password"))
	confirmPassword := strings.TrimSpace(r.FormValue("confirm_password"))

	if !authhandler.IsValidPassword(password) {
		htmx.ReSwap(w, "none")
		_ = h.Render(w, r, http.StatusUnprocessableEntity, toast.ToastMessage(toast.Error, "Please provide a valid password."))
		return
	}

	if password != confirmPassword {
		htmx.ReSwap(w, "none")
		_ = h.Render(w, r, http.StatusUnprocessableEntity, toast.ToastMessage(toast.Error, "Passwords do not match."))
		return
	}

	passwordHash, err := auth.Hash(password)
	if err != nil {
		h.Logger.Error("hash password failed", "err", err)
		htmx.ReSwap(w, "none")
		_ = h.Render(w, r, http.StatusInternalServerError, toast.ToastMessage(toast.Error, "Something went wrong."))
		return
	}

	if err := h.Auth.AddPassword(ctx, userID, passwordHash); err != nil {
		htmx.ReSwap(w, "none")

		msg := "Could not add password."
		status := http.StatusUnprocessableEntity

		switch {
		case errors.Is(err, auth.ErrPasswordAlreadyExists):
			msg = "A password is already set for this account."
		default:
			h.Logger.Error("add password failed", "err", err)
			status = http.StatusInternalServerError
			msg = "Something went wrong."
		}

		_ = h.Render(w, r, status, toast.ToastMessage(toast.Error, msg))
		return
	}

	cfg, err := h.accountConfig(ctx)
	if err != nil {
		http.Error(w, "could not load account", http.StatusInternalServerError)
		return
	}

	_ = render.IntoBody(
		h.Render,
		w,
		r,
		http.StatusOK,
		"/auth/account",
		templ.Join(
			userview.Account(cfg),
			toast.ToastMessage(toast.Success, "Password updated."),
		),
	)
}

func parseLinkProvider(raw string) (domain.Provider, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "google":
		return domain.ProviderGoogle, nil
	default:
		return "", auth.ErrUnsupportedProvider
	}
}
