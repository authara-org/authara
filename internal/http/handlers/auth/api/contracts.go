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

var DisableUserPostErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeInvalidRequest: {
		Status: http.StatusBadRequest,
		Code:   response.CodeInvalidRequest,
	},
	response.CodeInternalError: {
		Status: http.StatusInternalServerError,
		Code:   response.CodeInternalError,
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

var APIRouteSpecs = []RouteContractSpec{
	{
		Method: http.MethodGet,
		Path:   "/auth/api/v1/user",
		Errors: UserGetErrors,
	},
	{
		Method: http.MethodPost,
		Path:   "/auth/api/v1/users/{userID}/disable",
		Errors: DisableUserPostErrors,
	},
	{
		Method: http.MethodPost,
		Path:   "/auth/api/v1/refresh",
		Errors: RefreshPostErrors,
	},
}
