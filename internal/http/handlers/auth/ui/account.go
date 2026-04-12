package ui

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	authhandler "github.com/authara-org/authara/internal/http/handlers/auth"
	"github.com/authara-org/authara/internal/http/kit/htmx"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/kit/render"
	authview "github.com/authara-org/authara/internal/http/templates/auth"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
	userview "github.com/authara-org/authara/internal/http/templates/user"
	"github.com/authara-org/authara/internal/session"
	"github.com/google/uuid"
)

func (h *UIHandler) AccountGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", "/auth/account"), http.StatusSeeOther)
		return
	}

	user, err := h.Auth.GetUser(ctx, userID)
	if err != nil {
		session.ClearSessionCookies(w)
		redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", "/auth/account"), http.StatusSeeOther)
		return
	}

	_ = h.Render(
		w,
		r,
		http.StatusOK,
		userview.Account(user.Username, user.Email, true),
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
		changeUsernameForm := userview.ChangeUsernameForm(user.Username)
		toastMessage := toast.ToastMessage(
			toast.Error,
			"Failed to load user",
		)

		_ = h.Render(
			w,
			r,
			http.StatusUnprocessableEntity,
			templ.Join(changeUsernameForm, toastMessage),
		)
		return
	}

	if err := r.ParseForm(); err != nil {
		changeUsernameForm := userview.ChangeUsernameForm(user.Username)
		toastMessage := toast.ToastMessage(
			toast.Error,
			"Failed to read new username",
		)

		_ = h.Render(
			w,
			r,
			http.StatusUnprocessableEntity,
			templ.Join(changeUsernameForm, toastMessage),
		)
		return
	}

	username := r.FormValue("username")
	username = strings.TrimSpace(username)

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

		_ = h.Render(
			w, r,
			status,
			templ.Join(
				userview.ChangeUsernameForm(user.Username),
				toast.ToastMessage(toast.Error, msg),
			),
		)
		return
	}

	_ = h.Render(
		w, r,
		http.StatusOK,
		templ.Join(
			userview.ChangeUsernameForm(username),
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

	challengeID, err := h.Challenge.CreateEmailChangeChallenge(
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

	user, err := h.Auth.GetUser(ctx, result.Action.UserID)
	if err != nil {
		session.ClearSessionCookies(w)
		redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", "/auth/account"), http.StatusSeeOther)
		return
	}

	c := templ.Join(
		userview.Account(user.Username, user.Email, true),
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
		toastMessage := toast.ToastMessage(
			toast.Error,
			"Error deleting Account",
		)

		_ = h.Render(
			w,
			r,
			http.StatusTooManyRequests,
			toastMessage,
		)
		return
	}

	session.ClearSessionCookies(w)
	redirect.Redirect(w, r, "/auth/successfull-deletion", http.StatusSeeOther)
}
