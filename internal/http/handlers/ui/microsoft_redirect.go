package ui

import (
	"net/http"

	authview "github.com/authara-org/authara/internal/http/templates/auth"
)

func (h *UIHandler) MicrosoftRedirectPage(w http.ResponseWriter, r *http.Request) {
	_ = h.Render(w, r, http.StatusOK, authview.MicrosoftRedirect())
}
