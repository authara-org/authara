package api

import (
	"net/http"

	httpcontext "github.com/alexlup06-authgate/authgate/internal/http/kit/context"
	"github.com/alexlup06-authgate/authgate/internal/http/kit/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *APIHandler) UserGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpcontext.UserID(ctx)
	if !ok {
		response.ErrorJSON(
			w,
			http.StatusUnauthorized,
			response.CodeUnauthorized,
			"Unauthorized",
		)
		return
	}

	user, err := h.Auth.GetUser(ctx, userID)
	if err != nil {
		response.ErrorJSON(
			w,
			http.StatusUnauthorized,
			response.CodeUnauthorized,
			"Unauthorized",
		)
		return
	}

	response.JSON(w, http.StatusOK, response.UserFromDomain(*user))
}

func (h *APIHandler) DisableUserPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
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
		response.ErrorJSON(
			w,
			http.StatusInternalServerError,
			response.CodeInternalError,
			"Server error",
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
