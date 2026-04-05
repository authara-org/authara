package api

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/response"
)

func (h *APIHandler) UserGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		response.WriteError(
			w,
			mustRouteError(UserGetErrors, response.CodeUnauthorized),
			"Unauthorized",
		)
		return
	}

	roles, ok := httpctx.Roles(ctx)
	if !ok {
		response.WriteError(
			w,
			mustRouteError(UserGetErrors, response.CodeUnauthorized),
			"Unauthorized",
		)
		return
	}

	user, err := h.Auth.GetUser(ctx, userID)
	if err != nil {
		response.WriteError(
			w,
			mustRouteError(UserGetErrors, response.CodeUnauthorized),
			"Unauthorized",
		)
		return
	}

	response.JSON(w, http.StatusOK, response.UserWithRoles(user, roles.List()))
}

func (h *APIHandler) ChangeUsername(w http.ResponseWriter, r *http.Request) {

}

func (h *APIHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {

}

// func (h *APIHandler) DisableUserPost(w http.ResponseWriter, r *http.Request) {
// 	ctx := r.Context()
//
// 	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
// 	if err != nil {
// 		response.WriteError(
// 			w,
// 			mustRouteError(DisableUserPostErrors, response.CodeInvalidRequest),
// 			"Invalid user ID",
// 		)
// 		return
// 	}
//
// 	err = h.Auth.DisableUser(ctx, userID)
// 	if err != nil {
// 		response.WriteError(
// 			w,
// 			mustRouteError(DisableUserPostErrors, response.CodeInternalError),
// 			"Server error",
// 		)
// 		return
// 	}
//
// 	w.WriteHeader(http.StatusNoContent)
// }
