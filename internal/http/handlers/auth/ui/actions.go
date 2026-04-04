package ui

import (
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
	"github.com/authara-org/authara/internal/http/kit/htmx"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/kit/response"
	authview "github.com/authara-org/authara/internal/http/templates/auth"
	challengeview "github.com/authara-org/authara/internal/http/templates/challenge"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
	userview "github.com/authara-org/authara/internal/http/templates/user"
	"github.com/authara-org/authara/internal/session"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type signupFormInput struct {
	Email    string
	Password string
}

func (h *UIHandler) SignupPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	form, err := h.parseSignupForm(r)
	if err != nil {
		h.renderFormError(w, r, http.StatusUnprocessableEntity, "Please provide a valid email and password.", authview.SignupForm())
		return
	}

	if !authhandler.IsValidEmail(form.Email) || !authhandler.IsValidPassword(form.Password) {
		h.renderFormError(w, r, http.StatusUnprocessableEntity, "Please provide a valid email and password.", authview.SignupForm())
		return
	}

	ip := httputil.ClientIP(r)
	allowed, err := h.Limiter.AllowSignupAttempt(ctx, ip, form.Email)
	if err != nil || !allowed {
		h.renderFormError(w, r, http.StatusTooManyRequests, "Too many attempts. Please try again later.", authview.SignupForm())
		return
	}

	if h.ChallengeEnabled {
		h.startSignupChallenge(w, r, form.Email, form.Password)
		return
	}

	passwordHash, err := auth.Hash(form.Password)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.finishSignup(
		w,
		r,
		auth.SignupInput{
			Provider:     domain.ProviderPassword,
			Email:        form.Email,
			PasswordHash: passwordHash,
		},
		authview.SignupForm(),
	)
}

func (h *UIHandler) parseSignupForm(r *http.Request) (*signupFormInput, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}

	email := strings.TrimSpace(r.FormValue("email"))
	email = strings.ToLower(email)

	return &signupFormInput{
		Email:    email,
		Password: r.FormValue("password"),
	}, nil
}

func (h *UIHandler) startSignupChallenge(
	w http.ResponseWriter,
	r *http.Request,
	email string,
	password string,
) {
	ctx := r.Context()

	passwordHash, err := auth.Hash(password)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	exists, err := h.Auth.UserExistsByEmail(ctx, email)
	if err != nil {
		h.renderFormError(
			w, r,
			http.StatusUnprocessableEntity,
			"Could not start signup verification. Please try again.",
			authview.SignupForm(),
		)
		return
	}
	if exists {
		// opaque challenge — no pending action, no email job
		challengeID, err := h.Challenge.CreateOpaqueChallenge(ctx, time.Now().UTC(), domain.ChallengePurposeSignup, email)
		if err != nil {
			h.renderFormError(
				w, r,
				http.StatusUnprocessableEntity,
				"Could not start signup verification. Please try again.",
				authview.SignupForm(),
			)
			return
		}

		htmx.ReTarget(w, "#body")
		htmx.ReSwap(w, "innerHTML")
		htmx.PushUrl(w, "/auth/verify-challenge?challenge_id="+challengeID.String()+"&return_to="+httpctx.ReturnToOrDefault(ctx, "/"))

		_ = h.Render(
			w,
			r,
			http.StatusOK,
			challengeview.VerifyChallenge(challengeID.String(), "Verify your Email"),
		)
		return
	}

	challengeID, err := h.Challenge.CreateSignupChallenge(ctx, challenge.CreateSignupChallengeInput{
		Email:        email,
		Username:     "",
		PasswordHash: passwordHash,
	}, time.Now().UTC())
	if err != nil {
		h.renderFormError(w, r, http.StatusUnprocessableEntity, "Could not start signup verification. Please try again.", authview.SignupForm())
		return
	}

	htmx.ReTarget(w, "#body")
	htmx.ReSwap(w, "innerHTML")
	htmx.PushUrl(w, "/auth/verify-challenge?challenge_id="+challengeID.String()+"&return_to="+httpctx.ReturnToOrDefault(ctx, "/"))

	_ = h.Render(
		w,
		r,
		http.StatusOK,
		challengeview.VerifyChallenge(challengeID.String(), "Verify your Email"),
	)
}

func (h *UIHandler) finishSignup(
	w http.ResponseWriter,
	r *http.Request,
	input auth.SignupInput,
	errorRenderForm templ.Component,
) {
	ctx := r.Context()

	user, err := h.Auth.Signup(ctx, input)
	if err != nil {
		h.renderFormError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"Could not create account. Please check your details.",
			errorRenderForm,
		)
		return
	}

	returnTo, ok := httpctx.ReturnTo(ctx)
	if !ok {
		returnTo = "/"
	}

	audience := redirect.AudienceForPath(returnTo)
	ua := r.UserAgent()
	now := time.Now()

	accessToken, refreshToken, err := h.Session.CreateSession(ctx, user.ID, audience, ua, now)
	if err != nil {
		h.renderFormError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"Did not create session.",
			errorRenderForm,
		)
		return
	}

	session.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(h.RefreshTTL.Seconds()))

	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (h *UIHandler) VerifyChallengePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	challengeIDStr := strings.TrimSpace(r.FormValue("challenge_id"))
	code := strings.TrimSpace(r.FormValue("code"))

	challengeID, err := uuid.Parse(challengeIDStr)
	if err != nil {
		h.renderFormError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"Invalid verification request.",
			challengeview.VerifyChallengeForm(challengeIDStr, true),
		)
		return
	}

	if len(code) != 6 {
		h.renderFormError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"Please enter the 6-digit verification code.",
			challengeview.VerifyChallengeForm(challengeIDStr, true),
		)
		return
	}

	result, err := h.Challenge.VerifySignupChallenge(
		ctx,
		challengeID,
		code,
		h.Verification,
		time.Now().UTC(),
	)
	if err != nil {
		msg := "Invalid or expired verification code."

		switch err {
		case challenge.ErrChallengeExpired:
			msg = "This verification code has expired."
		case challenge.ErrChallengeConsumed:
			msg = "This verification code has already been used."
		case challenge.ErrTooManyAttempts:
			msg = "Too many incorrect attempts. Please start again."
		case challenge.ErrInvalidVerificationCode:
			msg = "The verification code is incorrect."
		}

		h.renderFormError(
			w,
			r,
			http.StatusUnprocessableEntity,
			msg,
			challengeview.VerifyChallengeForm(challengeIDStr, true),
		)
		return
	}

	h.finishSignup(
		w,
		r,
		auth.SignupInput{
			Provider:     domain.ProviderPassword,
			Username:     result.Action.Username,
			Email:        result.Action.Email,
			PasswordHash: result.Action.PasswordHash,
		},
		challengeview.VerifyChallengeForm(challengeIDStr, true),
	)
}

func (h *UIHandler) ResendChallengePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	challengeIDStr := strings.TrimSpace(r.FormValue("challenge_id"))
	fmt.Println(challengeIDStr)
	challengeID, err := uuid.Parse(challengeIDStr)
	if err != nil {
		http.Error(w, "invalid challenge", http.StatusBadRequest)
		return
	}

	err = h.Challenge.ResendChallenge(ctx, challengeID, time.Now().UTC())
	if err != nil {
		msg := "Could not resend verification code."

		switch err {
		case challenge.ErrChallengeExpired:
			msg = "This verification request has expired."
		case challenge.ErrChallengeConsumed:
			msg = "This verification request has already been completed."
		case challenge.ErrTooManyResends:
			msg = "Too many resend attempts. Please start again."
		case challenge.ErrResendTooSoon:
			msg = "Please wait a moment before requesting another code."
		}

		_ = h.Render(
			w,
			r,
			http.StatusOK,
			toast.ToastMessage(toast.Error, msg),
		)
		return
	}

	_ = h.Render(
		w,
		r,
		http.StatusOK,
		toast.ToastMessage(toast.Success, "A new verification code has been sent."),
	)
}

func (h *UIHandler) LoginPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		h.renderFormError(w, r, http.StatusBadRequest, "Bad Form", authview.LoginForm())
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	email = strings.ToLower(email)
	password := r.FormValue("password")

	if email == "" || password == "" {
		h.renderFormError(w, r, http.StatusBadRequest, "Email and password required.", authview.LoginForm())
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
		h.renderFormError(w, r, http.StatusTooManyRequests, "Too many attempts. Please try again later.", authview.LoginForm())
		return
	}

	user, err := h.Auth.Login(ctx, input)
	if err != nil || user == nil {
		h.renderFormError(w, r, http.StatusUnprocessableEntity, "Invalid email or password.", authview.LoginForm())
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

func (h *UIHandler) renderFormError(w http.ResponseWriter, r *http.Request, status int, msg string, form templ.Component) {
	toastMessage := toast.ToastMessage(toast.Error, msg)

	_ = h.Render(
		w,
		r,
		status,
		templ.Join(form, toastMessage),
	)
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
