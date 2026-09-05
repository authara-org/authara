package ui

import (
	"net/http"

	"github.com/authara-org/authara/internal/email"
	operatorview "github.com/authara-org/authara/internal/http/templates/operator"
)

func (h *UIHandler) OperatorPage(w http.ResponseWriter, r *http.Request) {
	templates := email.TemplateCatalog()
	_ = h.Render(w, r, http.StatusOK, operatorview.Dashboard(len(templates)))
}

func (h *UIHandler) OperatorEmailTemplatesPage(w http.ResponseWriter, r *http.Request) {
	_ = h.Render(w, r, http.StatusOK, operatorview.EmailTemplates(email.TemplateCatalog()))
}
