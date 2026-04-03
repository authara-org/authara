package ui

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	authhandler "github.com/authara-org/authara/internal/http/handlers/auth"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/kit/response"
	authview "github.com/authara-org/authara/internal/http/templates/auth"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
	userview "github.com/authara-org/authara/internal/http/templates/user"
	"github.com/authara-org/authara/internal/session"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *UIHandler) SignupPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	email = strings.ToLower(email)
	password := r.FormValue("password")

	if !authhandler.IsValidEmail(email) || !authhandler.IsValidPassword(password) {
		signupForm := authview.SignupForm()
		toastMessage := toast.ToastMessage(
			toast.Error,
			"Please provide a valid email and password.",
		)

		_ = h.Render(
			w,
			r,
			http.StatusUnprocessableEntity,
			templ.Join(signupForm, toastMessage),
		)
		return
	}

	input := auth.SignupInput{
		Provider: domain.ProviderPassword,
		Email:    email,
		Password: password,
	}

	ip := httputil.ClientIP(r)
	allowed, err := h.Limiter.AllowSignupAttempt(ctx, ip, email)
	if err != nil || !allowed {
		signupForm := authview.SignupForm()
		toastMessage := toast.ToastMessage(
			toast.Error,
			"Too many attempts. Please try again later.",
		)

		_ = h.Render(
			w,
			r,
			http.StatusTooManyRequests,
			templ.Join(signupForm, toastMessage),
		)
		return
	}

	user, err := h.Auth.Signup(ctx, input)
	if err != nil {
		signupForm := authview.SignupForm()
		toastMessage := toast.ToastMessage(
			toast.Error,
			"Could not create account. Please check your details.",
		)

		_ = h.Render(
			w,
			r,
			http.StatusUnprocessableEntity,
			templ.Join(signupForm, toastMessage),
		)
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
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	session.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(h.RefreshTTL.Seconds()))

	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (h *UIHandler) VerifyChallengePost(w http.ResponseWriter, r *http.Request) {
}

func (h *UIHandler) ResendChallengePost(w http.ResponseWriter, r *http.Request) {
}

func (h *UIHandler) LoginPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	email = strings.ToLower(email)
	password := r.FormValue("password")

	if email == "" || password == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}

	input := auth.LoginInput{
		Provider: domain.ProviderPassword,
		Email:    email,
		Password: password,
	}

	ip := httputil.ClientIP(r)
	allowed, err := h.Limiter.AllowLoginAttempt(ctx, ip, email)
	if err != nil || !allowed {
		loginform := authview.LoginForm()
		toastMessage := toast.ToastMessage(
			toast.Error,
			"Too many attempts. Please try again later.",
		)

		_ = h.Render(
			w,
			r,
			http.StatusTooManyRequests,
			templ.Join(loginform, toastMessage),
		)
		return
	}

	user, err := h.Auth.Login(ctx, input)
	if err != nil || user == nil {
		loginForm := authview.LoginForm()
		toastMessage := toast.ToastMessage(
			toast.Error,
			"Invalid email or password.",
		)

		_ = h.Render(
			w,
			r,
			http.StatusUnprocessableEntity,
			templ.Join(loginForm, toastMessage),
		)
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
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	session.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(h.RefreshTTL.Seconds()))

	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (h *UIHandler) LogoutPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	refreshToken, exists := session.ReadRefreshToken(r)
	if exists {
		_ = h.Session.Logout(ctx, refreshToken)
	}

	session.ClearSessionCookies(w)

	returnTo, ok := httpctx.ReturnTo(r.Context())
	if !ok {
		returnTo = "/"
	}

	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (h *UIHandler) RefreshPost(w http.ResponseWriter, r *http.Request) {
	now := time.Now()

	returnTo, ok := httpctx.ReturnTo(r.Context())
	if !ok {
		returnTo = r.URL.Path
		if r.URL.RawQuery != "" {
			returnTo += "?" + r.URL.RawQuery
		}
	}

	audience := redirect.AudienceForPath(returnTo)
	refresh, ok := session.ReadRefreshToken(r)
	if !ok {
		session.ClearSessionCookies(w)
		redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", returnTo), http.StatusSeeOther)
		return
	}

	accessToken, newRefreshToken, err := h.Session.RefreshSession(
		r.Context(),
		refresh,
		audience,
		now,
	)
	if err != nil {
		session.ClearSessionCookies(w)
		redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", returnTo), http.StatusSeeOther)
		return
	}

	session.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(w, newRefreshToken, int(h.RefreshTTL.Seconds()))
	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
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

func (h *UIHandler) DisableUserPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		// TODO: Change to htmx reponse
		response.ErrorJSON(
			w,
			http.StatusBadRequest,
			response.CodeInvalidRequest,
			"Invalid user ID",
		)
		return
	}

	err = h.Auth.DisableUser(ctx, userID)
	if err != nil {
		// TODO: Change to htmx reponse
		response.ErrorJSON(
			w,
			http.StatusInternalServerError,
			response.CodeInternalError,
			"Server error",
		)
		return
	}

	// TODO: Change to htmx reponse
	w.WriteHeader(http.StatusNoContent)
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
