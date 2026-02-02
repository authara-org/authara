package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/alexlup06-authgate/authgate/internal/auth"
	"github.com/alexlup06-authgate/authgate/internal/domain"
	"github.com/alexlup06-authgate/authgate/internal/http/authflow"
	httpcontext "github.com/alexlup06-authgate/authgate/internal/http/context"
	"github.com/alexlup06-authgate/authgate/internal/http/csrf"
	"github.com/alexlup06-authgate/authgate/internal/http/providers/google"
	"github.com/alexlup06-authgate/authgate/internal/http/redirect"
	"github.com/alexlup06-authgate/authgate/internal/http/response"
	authview "github.com/alexlup06-authgate/authgate/internal/http/templates/auth"
	"github.com/alexlup06-authgate/authgate/internal/session"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
	if authflow.TryRedirectAuthenticated(w, r, h.session, h.accessTokenTTL, h.refreshTokenTTL) {
		return
	}

	tok, err := csrf.EnsureCookie(w, r)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	r = r.WithContext(httpcontext.WithCSRF(r.Context(), tok))

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

	tok, err := csrf.EnsureCookie(w, r)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	r = r.WithContext(httpcontext.WithCSRF(r.Context(), tok))

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

	_, err = csrf.EnsureCookie(w, r)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

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
		http.Error(w, "refresh token missing", http.StatusUnauthorized)
		return
	}

	audience, err := redirect.AudienceFromRequest(r)
	if err != nil {
		http.Error(w, "invalid audience", http.StatusBadRequest)
		return
	}

	now := time.Now()
	newAccessToken, newRefreshToken, err := h.session.RefreshSession(ctx, refreshToken, audience, now)
	switch {
	case errors.Is(err, session.ErrInvalidRefreshToken), errors.Is(err, session.ErrForbidden):
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return

	case err != nil:
		http.Error(w, "session error", http.StatusInternalServerError)
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
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.auth.GetUser(ctx, userID)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	response.JSON(w, http.StatusOK, user)
}

func (h *AuthHandler) DisableUserPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	err = h.auth.DisableUser(ctx, userID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
