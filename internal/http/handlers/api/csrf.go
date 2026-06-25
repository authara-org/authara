package api

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/csrf"
	"github.com/authara-org/authara/internal/http/kit/response"
)

func (h *APIHandler) CSRFGet(w http.ResponseWriter, r *http.Request) {
	token, err := csrf.EnsureCookie(w, r)
	if err != nil {
		response.WriteError(
			w,
			mustRouteError(CSRFGetErrors, response.CodeInternalError),
			"CSRF token error",
		)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"csrf_token": token})
}
