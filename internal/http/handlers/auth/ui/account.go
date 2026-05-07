package ui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/domain"
	authhandler "github.com/authara-org/authara/internal/http/handlers/auth"
	"github.com/authara-org/authara/internal/http/kit/flash"
	"github.com/authara-org/authara/internal/http/kit/htmx"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/kit/render"
	authview "github.com/authara-org/authara/internal/http/templates/auth"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
	userview "github.com/authara-org/authara/internal/http/templates/user"
	"github.com/authara-org/authara/internal/http/viewmodel"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

func (h *UIHandler) AccountGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	msg, _ := flash.Read(w, r)
	if msg != nil {
		r = r.WithContext(httpctx.WithFlash(r.Context(), msg))
	}

	accountCfg, err := h.accountConfig(ctx)
	if err != nil {
		session.ClearSessionCookies(w)
		redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", "/auth/account"), http.StatusSeeOther)
		return
	}

	_ = h.Render(
		w,
		r,
		http.StatusOK,
		userview.Account(accountCfg),
	)
}

func (h *UIHandler) AddPasswordPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.Auth.GetUser(ctx, userID)
	if err != nil {
		http.Error(w, "could not load user", http.StatusInternalServerError)
		return
	}

	ctx = httpctx.WithEmail(ctx, user.Email)

	_ = h.Render(
		w,
		r.WithContext(ctx),
		http.StatusOK,
		authview.AddPassword(),
	)
}

func (h *UIHandler) ChangePasswordPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.Auth.GetUser(ctx, userID)
	if err != nil {
		http.Error(w, "could not load user", http.StatusInternalServerError)
		return
	}

	ctx = httpctx.WithEmail(ctx, user.Email)

	_ = h.Render(
		w,
		r.WithContext(ctx),
		http.StatusOK,
		authview.ChangePassword(),
	)
}

func (h *UIHandler) SuccessfullDeletionPage(w http.ResponseWriter, r *http.Request) {
	_ = h.Render(
		w,
		r,
		http.StatusOK,
		authview.SuccessfullDeletion(),
	)
}

func (h *UIHandler) ChangeUsernamePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", "/auth/account"), http.StatusSeeOther)
		return
	}

	user, err := h.Auth.GetUser(ctx, userID)
	if err != nil {
		htmx.ReSwap(w, "none")
		_ = h.Render(
			w,
			r,
			http.StatusUnprocessableEntity,
			toast.ToastMessage(toast.Error, "Failed to load user"),
		)
		return
	}

	if err := r.ParseForm(); err != nil {
		htmx.ReSwap(w, "none")
		_ = h.Render(
			w,
			r,
			http.StatusUnprocessableEntity,
			templ.Join(
				userview.ChangeUsernameSection(user.Username),
				toast.ToastMessage(toast.Error, "Failed to read new username"),
			),
		)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))

	err = h.Auth.ChangeUsername(ctx, userID, username)
	if err != nil {
		status := http.StatusUnprocessableEntity
		msg := "Could not update username"

		switch {
		case errors.Is(err, auth.ErrUsernameTaken):
			msg = "Username is already taken"
		case errors.Is(err, auth.ErrInvalidUsername):
			msg = "Invalid username"
		default:
			h.Logger.Error("change username failed", "err", err)
			status = http.StatusInternalServerError
			msg = "Something went wrong"
		}

		htmx.ReSwap(w, "none")
		_ = h.Render(
			w,
			r,
			status,
			toast.ToastMessage(toast.Error, msg),
		)
		return
	}

	_ = h.Render(
		w,
		r,
		http.StatusOK,
		templ.Join(
			userview.ChangeUsernameSection(username),
			toast.ToastMessage(toast.Success, "Username updated"),
			userview.DisplayUsername(username, true),
		),
	)
}

func (h *UIHandler) EmailChangeRequestPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.Auth.GetUser(ctx, userID)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		htmx.ReSwap(w, "none")
		_ = h.Render(
			w,
			r,
			http.StatusUnprocessableEntity,
			toast.ToastMessage(toast.Error, "Please provide a valid email address."),
		)
		return
	}

	newEmail := strings.TrimSpace(r.FormValue("new_email"))
	newEmail = strings.ToLower(newEmail)

	if !authhandler.IsValidEmail(newEmail) {
		htmx.ReSwap(w, "none")
		_ = h.Render(
			w,
			r,
			http.StatusUnprocessableEntity,
			toast.ToastMessage(toast.Error, "Please provide a valid email address."),
		)
		return
	}

	if strings.EqualFold(user.Email, newEmail) {
		htmx.ReSwap(w, "none")
		_ = h.Render(
			w,
			r,
			http.StatusUnprocessableEntity,
			toast.ToastMessage(toast.Error, "Please enter a different email address."),
		)
		return
	}

	var challengeID uuid.UUID

	exists, err := h.Auth.UserExistsByEmail(ctx, newEmail)
	if err != nil {
		htmx.ReSwap(w, "none")
		_ = h.Render(
			w,
			r,
			http.StatusUnprocessableEntity,
			toast.ToastMessage(toast.Error, "Could not start email change. Please try again."),
		)
		return
	}

	if exists {
		// Opaque challenge to avoid revealing whether the email already exists.
		challengeID, err = h.Challenge.CreateOpaqueChallenge(
			ctx,
			time.Now().UTC(),
			domain.ChallengePurposeEmailChange,
			newEmail,
		)
		if err != nil {
			htmx.ReSwap(w, "none")
			_ = h.Render(
				w,
				r,
				http.StatusUnprocessableEntity,
				toast.ToastMessage(toast.Error, "Could not start email change. Please try again."),
			)
			return
		}
	} else {
		challengeID, err = h.Challenge.CreateEmailChangeChallenge(
			ctx,
			challenge.CreateEmailChangeChallengeInput{
				UserID:   user.ID,
				OldEmail: user.Email,
				NewEmail: newEmail,
			},
			time.Now().UTC(),
		)
		if err != nil {
			htmx.ReSwap(w, "none")
			_ = h.Render(
				w,
				r,
				http.StatusUnprocessableEntity,
				toast.ToastMessage(toast.Error, "Could not start email change. Please try again."),
			)
			return
		}
	}

	_ = h.renderVerifyChallengeRedirect(
		w,
		r,
		VerifyChallengeActionEmailChange,
		challengeID.String(),
		httpctx.ReturnToOrDefault(ctx, "/auth/account"),
	)
}

func (h *UIHandler) verifyEmailChangeChallengePost(
	w http.ResponseWriter,
	r *http.Request,
	challengeIDStr string,
	challengeID uuid.UUID,
	code string,
) {
	ctx := r.Context()

	result, err := h.Challenge.VerifyEmailChangeChallenge(
		ctx,
		challengeID,
		code,
		h.Verification,
		time.Now().UTC(),
	)
	if err != nil {
		h.renderVerifyChallengeError(
			w,
			r,
			VerifyChallengeActionEmailChange,
			challengeIDStr,
			h.verifyChallengeErrorMessage(err),
		)
		return
	}

	if err := h.Challenge.ExecuteEmailChange(ctx, result.Action, time.Now().UTC()); err != nil {
		h.renderVerifyChallengeError(
			w,
			r,
			VerifyChallengeActionEmailChange,
			challengeIDStr,
			"Could not change email. Please try again.",
		)
		return
	}

	accountCfg, err := h.accountConfig(ctx)
	if err != nil {
		session.ClearSessionCookies(w)
		redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", "/auth/account"), http.StatusSeeOther)
		return
	}

	c := templ.Join(
		userview.Account(accountCfg),
		toast.ToastMessage(toast.Success, "Your email has been changed."),
	)

	_ = render.IntoBody(
		h.Render,
		w,
		r,
		http.StatusOK,
		httpctx.ReturnToOrDefault(ctx, "/auth/account"),
		c,
	)
}

func (h *UIHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", "/auth/account"), http.StatusSeeOther)
		return
	}

	err := h.Auth.DeleteUser(ctx, userID)
	if err != nil {
		_ = h.Render(
			w,
			r,
			http.StatusTooManyRequests,
			toast.ToastMessage(toast.Error, "Error deleting Account"),
		)
		return
	}

	session.ClearSessionCookies(w)
	redirect.Redirect(w, r, "/auth/successful-deletion", http.StatusSeeOther)
}

func (h *UIHandler) accountConfig(ctx context.Context) (userview.AccountConfig, error) {
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return userview.AccountConfig{}, errors.New("missing user id")
	}

	user, err := h.Auth.GetUser(ctx, userID)
	if err != nil {
		return userview.AccountConfig{}, err
	}

	currentSessionID, _ := httpctx.SessionID(ctx)

	sessions, err := h.Session.ListUserSessions(ctx, userID, currentSessionID, time.Now().UTC())
	if err != nil {
		return userview.AccountConfig{}, err
	}

	providers, err := h.Auth.ListUserAuthProviders(ctx, userID)
	if err != nil {
		return userview.AccountConfig{}, err
	}

	return userview.AccountConfig{
		Username:         user.Username,
		Email:            user.Email,
		GoogleClientID:   h.Google.ClientID,
		Sessions:         toSessionViewModels(sessions, currentSessionID),
		CurrentSessionID: currentSessionID,
		AuthProviders:    viewmodel.AuthProvidersFromDomain(providers, h.OAuthProviders.Providers),
	}, nil
}

func (h *UIHandler) PasswordChangePost(w http.ResponseWriter, r *http.Request) {
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

	currentPassword := strings.TrimSpace(r.FormValue("current_password"))
	newPassword := strings.TrimSpace(r.FormValue("new_password"))
	confirmPassword := strings.TrimSpace(r.FormValue("confirm_password"))

	if !authhandler.IsValidPassword(newPassword) {
		htmx.ReSwap(w, "none")
		_ = h.Render(w, r, http.StatusUnprocessableEntity, toast.ToastMessage(toast.Error, "Please provide a valid new password."))
		return
	}

	if newPassword != confirmPassword {
		htmx.ReSwap(w, "none")
		_ = h.Render(w, r, http.StatusUnprocessableEntity, toast.ToastMessage(toast.Error, "Passwords do not match."))
		return
	}

	newPasswordHash, err := auth.Hash(newPassword)
	if err != nil {
		h.Logger.Error("hash password failed", "err", err)
		htmx.ReSwap(w, "none")
		_ = h.Render(w, r, http.StatusInternalServerError, toast.ToastMessage(toast.Error, "Something went wrong."))
		return
	}

	if err := h.Auth.ChangePassword(ctx, userID, currentPassword, newPasswordHash); err != nil {
		htmx.ReSwap(w, "none")

		msg := "Could not change password."
		status := http.StatusUnprocessableEntity

		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			msg = "Current password is incorrect."
		case errors.Is(err, store.ErrorAuthProviderNotFound):
			msg = "No password is set for this account."
		default:
			h.Logger.Error("change password failed", "err", err)
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

func parseProvider(s string) (domain.Provider, error) {
	switch s {
	case "google":
		return domain.ProviderGoogle, nil
	case "password":
		return domain.ProviderPassword, nil
	default:
		return "", fmt.Errorf("unknown provider")
	}
}
