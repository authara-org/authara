package ui

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
)

func (h *UIHandler) renderFormError(w http.ResponseWriter, r *http.Request, status int, msg string, form templ.Component) {
	toastMessage := toast.ToastMessage(toast.Error, msg)

	_ = h.Render(
		w,
		r,
		status,
		templ.Join(form, toastMessage),
	)
}
