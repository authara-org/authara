package ui

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/alexlup06-authgate/authgate/internal/http/kit/authflow"
	httpcontext "github.com/alexlup06-authgate/authgate/internal/http/kit/context"
	"github.com/alexlup06-authgate/authgate/internal/http/kit/redirect"
	"github.com/alexlup06-authgate/authgate/internal/http/kit/render"
	authview "github.com/alexlup06-authgate/authgate/internal/http/templates/auth"
	"github.com/alexlup06-authgate/authgate/internal/http/templates/components/toast"
	userview "github.com/alexlup06-authgate/authgate/internal/http/templates/user"
)

func (h *UIHandler) SignupPage(w http.ResponseWriter, r *http.Request) {
	if authflow.TryRedirectAuthenticated(w, r, h.Session, h.AccessTTL, h.RefreshTTL) {
		return
	}

	_ = render.Render(
		w,
		r,
		http.StatusOK,
		authview.Signup(),
	)
}

func (h *UIHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	if authflow.TryRedirectAuthenticated(w, r, h.Session, h.AccessTTL, h.RefreshTTL) {
		return
	}

	_ = render.Render(
		w,
		r,
		http.StatusOK,
		authview.Login(),
	)
}

func (h *UIHandler) AccountGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpcontext.UserID(ctx)
	if !ok {
		redirect.Redirect(w, r, "/auth/login?return_to=/auth/user", http.StatusSeeOther)
		return
	}

	user, err := h.Auth.GetUser(ctx, userID)
	if err != nil {
		changeUsernameForm := userview.ChangeUsernameForm(user.Email)
		toastMessage := toast.ToastMessage(
			toast.Error,
			"Failed to load user",
		)

		_ = render.Render(
			w,
			r,
			http.StatusUnprocessableEntity,
			templ.Join(changeUsernameForm, toastMessage),
		)
		return
	}

	_ = render.Render(
		w,
		r,
		http.StatusOK,
		userview.Account(user.Username, user.Email, true),
	)
}
