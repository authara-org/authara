package api

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *APIHandler) UserGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		response.ErrorJSON(
			w,
			http.StatusUnauthorized,
			response.CodeUnauthorized,
			"Unauthorized",
		)
		return
	}

	cu, err := h.Auth.GetCurrentUser(ctx, userID)
	if err != nil {
		response.ErrorJSON(
			w,
			http.StatusUnauthorized,
			response.CodeUnauthorized,
			"Unauthorized",
		)
		return
	}

	response.JSON(w, http.StatusOK, response.UserWithRoles(cu.User, cu.Roles))
}

func (h *APIHandler) ChangeUsername(w http.ResponseWriter, r *http.Request) {

}

func (h *APIHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {

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
