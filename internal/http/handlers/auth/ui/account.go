package ui

import (
	"errors"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	authview "github.com/authara-org/authara/internal/http/templates/auth"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
	userview "github.com/authara-org/authara/internal/http/templates/user"
	"github.com/authara-org/authara/internal/session"
)

func (h *UIHandler) AccountGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", "/auth/user"), http.StatusSeeOther)
		return
	}

	user, err := h.Auth.GetUser(ctx, userID)
	if err != nil || user == nil {
		session.ClearSessionCookies(w)
		redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", "/auth/user"), http.StatusSeeOther)
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
		redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", "/auth/user"), http.StatusSeeOther)
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
			userview.DisplayUsername(username),
		),
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
