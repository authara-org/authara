package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/alexlup06/authgate/internal/auth"
	"github.com/alexlup06/authgate/internal/http/providers/google"
	authview "github.com/alexlup06/authgate/internal/http/templates/auth"
	"github.com/alexlup06/authgate/internal/session"
)

type AuthHandlerConfig struct {
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type AuthHandler struct {
	auth            *auth.Service
	session         *session.Service
	google          *google.Client
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func NewAuthHandler(
	authService *auth.Service,
	sessionService *session.Service,
	google *google.Client,
	cfg AuthHandlerConfig,
) *AuthHandler {
	return &AuthHandler{
		auth:            authService,
		session:         sessionService,
		google:          google,
		accessTokenTTL:  cfg.AccessTokenTTL,
		refreshTokenTTL: cfg.RefreshTokenTTL,
	}
}

func (h *AuthHandler) SignupPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()

	refreshToken, ok := session.ReadRefreshToken(r)
	if ok {
		access, newRefresh, err := h.session.RefreshSession(
			ctx,
			refreshToken,
			now,
		)
		if err == nil {
			session.SetAccessToken(w, access, int(h.accessTokenTTL.Seconds()))
			session.SetRefreshToken(w, newRefresh, int(h.refreshTokenTTL.Seconds()))

			http.Redirect(w, r, "/", http.StatusSeeOther)

			return
		}

		session.ClearSessionCookies(w)
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

	if email == "" || password == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}

	input := auth.SignupInput{
		Provider: auth.ProviderPassword,
		Email:    email,
		Password: password,
	}

	user, err := h.auth.Signup(ctx, input)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	ua := r.UserAgent()
	now := time.Now()
	accessToken, refreshToken, err := h.session.CreateSession(ctx, user.ID, ua, now)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	session.SetAccessToken(w, accessToken, int(h.accessTokenTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(h.refreshTokenTTL.Seconds()))

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()

	refreshToken, ok := session.ReadRefreshToken(r)
	if ok {
		access, newRefresh, err := h.session.RefreshSession(
			ctx,
			refreshToken,
			now,
		)
		if err == nil {
			session.SetAccessToken(w, access, int(h.accessTokenTTL.Seconds()))
			session.SetRefreshToken(w, newRefresh, int(h.refreshTokenTTL.Seconds()))

			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		session.ClearSessionCookies(w)
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
		Provider: auth.ProviderPassword,
		Email:    email,
		Password: password,
	}

	user, err := h.auth.Login(ctx, input)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	ua := r.UserAgent()
	now := time.Now()
	accessToken, refreshToken, err := h.session.CreateSession(ctx, user.ID, ua, now)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	session.SetAccessToken(w, accessToken, int(h.accessTokenTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(h.refreshTokenTTL.Seconds()))

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {

}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	refreshToken, exists := session.ReadRefreshToken(r)
	if exists {
		_ = h.session.Logout(ctx, refreshToken)
	}

	session.ClearSessionCookies(w)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
