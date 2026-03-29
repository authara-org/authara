package ui

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/handlers/auth/ui/flow"
	"github.com/authara-org/authara/internal/http/kit/flash"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	authview "github.com/authara-org/authara/internal/http/templates/auth"
	userview "github.com/authara-org/authara/internal/http/templates/user"
	"github.com/authara-org/authara/internal/session"
)

func (h *UIHandler) SignupPage(w http.ResponseWriter, r *http.Request) {
	if flow.TryRedirectAuthenticated(w, r, h.Session, h.AccessTTL, h.RefreshTTL) {
		return
	}

	_ = h.Render(
		w,
		r,
		http.StatusOK,
		authview.Signup(h.OAuthProviders.Providers),
	)
}

func (h *UIHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	if flow.TryRedirectAuthenticated(w, r, h.Session, h.AccessTTL, h.RefreshTTL) {
		return
	}

	msg, _ := flash.Read(w, r)
	if msg != nil {
		r = r.WithContext(httpctx.WithFlash(r.Context(), msg))
	}

	_ = h.Render(
		w,
		r,
		http.StatusOK,
		authview.Login(h.OAuthProviders.Providers),
	)
}

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
