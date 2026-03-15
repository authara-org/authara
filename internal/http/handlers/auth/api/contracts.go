package api

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/response"
)

type RouteContractSpec struct {
	Method string
	Path   string
	Errors map[response.ErrorCode]response.ErrorSpec
}

var UserGetErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeUnauthorized: {
		Status: http.StatusUnauthorized,
		Code:   response.CodeUnauthorized,
	},
}

var RefreshPostErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeUnauthorized: {
		Status: http.StatusUnauthorized,
		Code:   response.CodeUnauthorized,
	},
	response.CodeInvalidRequest: {
		Status: http.StatusBadRequest,
		Code:   response.CodeInvalidRequest,
	},
	response.CodeInternalError: {
		Status: http.StatusInternalServerError,
		Code:   response.CodeInternalError,
	},
}

var LogoutPostErrors = map[response.ErrorCode]response.ErrorSpec{}

var APIRouteSpecs = []RouteContractSpec{
	{
		Method: http.MethodGet,
		Path:   "/auth/api/v1/user",
		Errors: UserGetErrors,
	},
	{
		Method: http.MethodPost,
		Path:   "/auth/api/v1/sessions/refresh",
		Errors: RefreshPostErrors,
	},
	{
		Method: http.MethodPost,
		Path:   "/auth/api/v1/sessions/logout",
		Errors: LogoutPostErrors,
	},
}
