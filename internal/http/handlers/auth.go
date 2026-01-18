package handlers

import (
	"net/http"
	"strings"

	"github.com/alexlup06/authgate/internal/auth"
	"github.com/alexlup06/authgate/internal/http/providers/google"
	authview "github.com/alexlup06/authgate/internal/http/templates/auth"
	"github.com/alexlup06/authgate/internal/session"
)

type AuthHandler struct {
	auth    *auth.Service
	session *session.Service
	google  *google.Client
}

func NewAuthHandler(
	authService *auth.Service,
	sessionService *session.Service,
	google *google.Client,
) *AuthHandler {
	return &AuthHandler{
		auth:    authService,
		session: sessionService,
		google:  google,
	}
}

func (h *AuthHandler) SignupPage(w http.ResponseWriter, r *http.Request) {
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

	_, err = h.session.Create(ctx, *user)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
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

	_, err = h.session.Create(ctx, *user)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	// h.session.SetCookie(w, session)
	// http.Redirect(w, r, "/", http.StatusFound)
}

func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {

}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("logout action (placeholder)"))
}
