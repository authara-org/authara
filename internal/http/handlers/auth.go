package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/alexlup06-authgate/authgate/internal/auth"
	"github.com/alexlup06-authgate/authgate/internal/domain"
	"github.com/alexlup06-authgate/authgate/internal/http/authflow"
	httpcontext "github.com/alexlup06-authgate/authgate/internal/http/context"
	"github.com/alexlup06-authgate/authgate/internal/http/providers/google"
	"github.com/alexlup06-authgate/authgate/internal/http/redirect"
	"github.com/alexlup06-authgate/authgate/internal/http/response"
	authview "github.com/alexlup06-authgate/authgate/internal/http/templates/auth"
	"github.com/alexlup06-authgate/authgate/internal/http/templates/components/toast"
	userview "github.com/alexlup06-authgate/authgate/internal/http/templates/user"
	"github.com/alexlup06-authgate/authgate/internal/ratelimit"
	"github.com/alexlup06-authgate/authgate/internal/session"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AuthHandlerConfig struct {
	AuthService    *auth.Service
	SessionService *session.Service
	Limiter        ratelimit.AuthLimiter
	Logger         *slog.Logger
	Google         *google.Client

	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type AuthHandler struct {
	auth            *auth.Service
	session         *session.Service
	limiter         ratelimit.AuthLimiter
	logger          *slog.Logger
	google          *google.Client
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func NewAuthHandler(cfg AuthHandlerConfig) *AuthHandler {
	return &AuthHandler{
		auth:            cfg.AuthService,
		session:         cfg.SessionService,
		limiter:         cfg.Limiter,
		logger:          cfg.Logger,
		google:          cfg.Google,
		accessTokenTTL:  cfg.AccessTokenTTL,
		refreshTokenTTL: cfg.RefreshTokenTTL,
	}
}

func (h *AuthHandler) SignupPage(w http.ResponseWriter, r *http.Request) {
	if authflow.TryRedirectAuthenticated(w, r, h.session, h.accessTokenTTL, h.refreshTokenTTL) {
		return
	}

	_ = Render(
		w,
		r,
		http.StatusOK,
		authview.Signup(),
	)
}

func (h *AuthHandler) SignupPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	email = strings.ToLower(email)
	password := r.FormValue("password")

	if !isValidEmail(email) || !isValidPassword(password) {
		signupForm := authview.SignupForm()
		toastMessage := toast.ToastMessage(
			toast.Error,
			"Please provide a valid email and password.",
		)

		_ = Render(
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

	ip := clientIP(r)
	allowed, err := h.limiter.AllowSignupAttempt(ctx, ip, email)
	if err != nil || !allowed {
		signupForm := authview.SignupForm()
		toastMessage := toast.ToastMessage(
			toast.Error,
			"Too many attempts. Please try again later.",
		)

		_ = Render(
			w,
			r,
			http.StatusTooManyRequests,
			templ.Join(signupForm, toastMessage),
		)
		return
	}

	user, err := h.auth.Signup(ctx, input)
	if err != nil {
		signupForm := authview.SignupForm()
		toastMessage := toast.ToastMessage(
			toast.Error,
			"Could not create account. Please check your details.",
		)

		_ = Render(
			w,
			r,
			http.StatusUnprocessableEntity,
			templ.Join(signupForm, toastMessage),
		)
		return
	}

	returnTo, ok := httpcontext.ReturnTo(r.Context())
	if !ok {
		returnTo = "/"
	}

	audience := redirect.AudienceForPath(returnTo)
	ua := r.UserAgent()
	now := time.Now()
	accessToken, refreshToken, err := h.session.CreateSession(ctx, user.ID, audience, ua, now)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	session.SetAccessToken(w, accessToken, int(h.accessTokenTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(h.refreshTokenTTL.Seconds()))

	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	if authflow.TryRedirectAuthenticated(w, r, h.session, h.accessTokenTTL, h.refreshTokenTTL) {
		return
	}

	_ = Render(
		w,
		r,
		http.StatusOK,
		authview.Login(),
	)
}

func (h *AuthHandler) LoginPost(w http.ResponseWriter, r *http.Request) {
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

	ip := clientIP(r)
	allowed, err := h.limiter.AllowSignupAttempt(ctx, ip, email)
	if err != nil || !allowed {
		loginform := authview.LoginForm()
		toastMessage := toast.ToastMessage(
			toast.Error,
			"Too many attempts. Please try again later.",
		)

		_ = Render(
			w,
			r,
			http.StatusTooManyRequests,
			templ.Join(loginform, toastMessage),
		)
		return
	}

	user, err := h.auth.Login(ctx, input)
	if err != nil || user == nil {
		loginForm := authview.LoginForm()
		toastMessage := toast.ToastMessage(
			toast.Error,
			"Invalid email or password.",
		)

		_ = Render(
			w,
			r,
			http.StatusUnprocessableEntity,
			templ.Join(loginForm, toastMessage),
		)
		return
	}

	returnTo, ok := httpcontext.ReturnTo(r.Context())
	if !ok {
		returnTo = "/"
	}

	audience := redirect.AudienceForPath(returnTo)
	ua := r.UserAgent()
	now := time.Now()
	accessToken, refreshToken, err := h.session.CreateSession(ctx, user.ID, audience, ua, now)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	session.SetAccessToken(w, accessToken, int(h.accessTokenTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(h.refreshTokenTTL.Seconds()))

	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	idToken := r.FormValue("credential")
	gtoken := r.FormValue("g_csrf_token")

	if idToken == "" || gtoken == "" {
		http.Error(w, "tokens are empty", http.StatusBadRequest)
		return
	}

	csrfCookie, err := r.Cookie("g_csrf_token")
	if err != nil {
		http.Error(w, "invalid csfr cookie", http.StatusUnauthorized)
		return
	}

	if gtoken == "" || gtoken != csrfCookie.Value {
		http.Error(w, "invalid g_csfr_token", http.StatusUnauthorized)
		return
	}

	identity, err := h.google.VerifyIDToken(ctx, idToken)
	if err != nil {
		http.Error(w, "invalid id token", http.StatusUnauthorized)
		return
	}

	input := auth.LoginInput{
		Provider: domain.ProviderGoogle,
		Email:    identity.Email,
		OAuthID:  identity.OAuthID,
	}

	user, err := h.auth.Login(ctx, input)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	returnTo, ok := httpcontext.ReturnTo(r.Context())
	if !ok {
		returnTo = "/"
	}

	audience := redirect.AudienceForPath(returnTo)
	ua := r.UserAgent()
	now := time.Now()
	accessToken, refreshToken, err := h.session.CreateSession(ctx, user.ID, audience, ua, now)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	session.SetAccessToken(w, accessToken, int(h.accessTokenTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(h.refreshTokenTTL.Seconds()))

	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (h *AuthHandler) LogoutPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	refreshToken, exists := session.ReadRefreshToken(r)
	if exists {
		_ = h.session.Logout(ctx, refreshToken)
	}

	session.ClearSessionCookies(w)

	returnTo, ok := httpcontext.ReturnTo(r.Context())
	if !ok {
		returnTo = "/"
	}

	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (h *AuthHandler) RefreshPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	refreshToken, ok := session.ReadRefreshToken(r)
	if !ok || refreshToken == "" {
		response.ErrorJSON(
			w,
			http.StatusUnauthorized,
			response.CodeUnauthorized,
			"Refresh token missing",
		)
		return
	}

	audience, err := redirect.AudienceFromRequest(r)
	if err != nil {
		response.ErrorJSON(
			w,
			http.StatusBadRequest,
			response.CodeInvalidRequest,
			"Invalid audience",
		)

		return
	}

	now := time.Now()
	newAccessToken, newRefreshToken, err := h.session.RefreshSession(ctx, refreshToken, audience, now)

	switch {
	case errors.Is(err, session.ErrInvalidRefreshToken),
		errors.Is(err, session.ErrForbidden):
		response.ErrorJSON(
			w,
			http.StatusUnauthorized,
			response.CodeUnauthorized,
			"Invalid refresh token",
		)
		return

	case err != nil:
		response.ErrorJSON(
			w,
			http.StatusInternalServerError,
			response.CodeInternalError,
			"Session error",
		)
		return
	}

	session.SetAccessToken(w, newAccessToken, int(h.accessTokenTTL.Seconds()))
	session.SetRefreshToken(w, newRefreshToken, int(h.refreshTokenTTL.Seconds()))

	w.WriteHeader(http.StatusOK)
}

func (h *AuthHandler) UserGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpcontext.UserID(ctx)
	if !ok {
		response.ErrorJSON(
			w,
			http.StatusUnauthorized,
			response.CodeUnauthorized,
			"Unauthorized",
		)
		return
	}

	user, err := h.auth.GetUser(ctx, userID)
	if err != nil {
		response.ErrorJSON(
			w,
			http.StatusUnauthorized,
			response.CodeUnauthorized,
			"Unauthorized",
		)
		return
	}

	response.JSON(w, http.StatusOK, response.UserFromDomain(*user))
}

func (h *AuthHandler) AccountGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpcontext.UserID(ctx)
	if !ok {
		redirect.Redirect(w, r, "/auth/login?return_to=/auth/user", http.StatusSeeOther)
		return
	}

	user, err := h.auth.GetUser(ctx, userID)
	if err != nil {
		changeUsernameForm := userview.ChangeUsernameForm(user.Email)
		toastMessage := toast.ToastMessage(
			toast.Error,
			"Failed to load user",
		)

		_ = Render(
			w,
			r,
			http.StatusUnprocessableEntity,
			templ.Join(changeUsernameForm, toastMessage),
		)
		return
	}

	_ = Render(
		w,
		r,
		http.StatusOK,
		userview.Account(user.Username, user.Email, true),
	)
}

func (h *AuthHandler) ChangeUsernamePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpcontext.UserID(ctx)
	if !ok {
		redirect.Redirect(w, r, "/auth/login?return_to=/auth/user", http.StatusSeeOther)
		return
	}

	user, err := h.auth.GetUser(ctx, userID)
	if err != nil {
		changeUsernameForm := userview.ChangeUsernameForm(user.Username)
		toastMessage := toast.ToastMessage(
			toast.Error,
			"Failed to load user",
		)

		_ = Render(
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

		_ = Render(
			w,
			r,
			http.StatusUnprocessableEntity,
			templ.Join(changeUsernameForm, toastMessage),
		)
		return
	}

	username := r.FormValue("username")
	username = strings.TrimSpace(username)

	err = h.auth.ChangeUsername(ctx, userID, username)
	if err != nil {
		status := http.StatusUnprocessableEntity
		msg := "Could not update username"

		switch {
		case errors.Is(err, auth.ErrUsernameTaken):
			msg = "Username is already taken"
		case errors.Is(err, auth.ErrInvalidUsername):
			msg = "Invalid username"
		default:
			h.logger.Error("change username failed", "err", err)
			status = http.StatusInternalServerError
			msg = "Something went wrong"
		}

		_ = Render(
			w, r,
			status,
			templ.Join(
				userview.ChangeUsernameForm(user.Username),
				toast.ToastMessage(toast.Error, msg),
			),
		)
		return
	}

	_ = Render(
		w, r,
		http.StatusOK,
		templ.Join(
			userview.ChangeUsernameForm(username),
			toast.ToastMessage(toast.Success, "Username updated"),
			userview.DisplayUsername(username),
		),
	)
}

func (h *AuthHandler) DisableUserPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		response.ErrorJSON(
			w,
			http.StatusBadRequest,
			response.CodeInvalidRequest,
			"Invalid user ID",
		)
		return
	}

	err = h.auth.DisableUser(ctx, userID)
	if err != nil {
		response.ErrorJSON(
			w,
			http.StatusInternalServerError,
			response.CodeInternalError,
			"Server error",
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
