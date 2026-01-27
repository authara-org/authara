package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/alexlup06/authgate/internal/auth"
	"github.com/alexlup06/authgate/internal/domain"
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
		Provider: domain.ProviderPassword,
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

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {

	access, ok := session.ReadAccessToken(r)
	if ok {
		_, err := h.session.ValidateAccessToken(
			r.Context(),
			access,
			time.Now(),
		)
		if err == nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

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
		Provider: domain.ProviderPassword,
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

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
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

	ua := r.UserAgent()
	now := time.Now()
	accessToken, refreshToken, err := h.session.CreateSession(ctx, user.ID, ua, now)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	session.SetAccessToken(w, accessToken, int(h.accessTokenTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(h.refreshTokenTTL.Seconds()))

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
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
