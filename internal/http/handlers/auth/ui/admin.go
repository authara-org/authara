package ui

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *UIHandler) DisableUserPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		// TODO: Change to htmx reponse
		response.ErrorJSON(
			w,
			http.StatusBadRequest,
			response.CodeInvalidRequest,
			"Invalid user ID",
		)
		return
	}

	err = h.Auth.DisableUser(ctx, userID)
	if err != nil {
		// TODO: Change to htmx reponse
		response.ErrorJSON(
			w,
			http.StatusInternalServerError,
			response.CodeInternalError,
			"Server error",
		)
		return
	}

	// TODO: Change to htmx reponse
	w.WriteHeader(http.StatusNoContent)
}
