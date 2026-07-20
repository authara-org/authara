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

const (
	codeAccountLinkRequired        response.ErrorCode = "account_link_required"
	codePasskeyAlreadyExists       response.ErrorCode = "passkey_already_exists"
	codePasskeyRegistrationInvalid response.ErrorCode = "passkey_registration_invalid"
)

var GoogleOptionsGetErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeNotFound: {
		Status: http.StatusNotFound,
		Code:   response.CodeNotFound,
	},
	response.CodeInternalError: {
		Status: http.StatusInternalServerError,
		Code:   response.CodeInternalError,
	},
}

var GoogleLoginPostErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeInvalidRequest: {
		Status: http.StatusBadRequest,
		Code:   response.CodeInvalidRequest,
	},
	response.CodeUnauthorized: {
		Status: http.StatusUnauthorized,
		Code:   response.CodeUnauthorized,
	},
	response.CodeForbidden: {
		Status: http.StatusForbidden,
		Code:   response.CodeForbidden,
	},
	response.CodeNotFound: {
		Status: http.StatusNotFound,
		Code:   response.CodeNotFound,
	},
	codeAccountLinkRequired: {
		Status: http.StatusConflict,
		Code:   codeAccountLinkRequired,
	},
	response.CodeInternalError: {
		Status: http.StatusInternalServerError,
		Code:   response.CodeInternalError,
	},
}

var UserGetErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeUnauthorized: {
		Status: http.StatusUnauthorized,
		Code:   response.CodeUnauthorized,
	},
}

var OrganizationsGetErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeUnauthorized: {
		Status: http.StatusUnauthorized,
		Code:   response.CodeUnauthorized,
	},
	response.CodeInternalError: {
		Status: http.StatusInternalServerError,
		Code:   response.CodeInternalError,
	},
}

var OrganizationMembersGetErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeUnauthorized: {
		Status: http.StatusUnauthorized,
		Code:   response.CodeUnauthorized,
	},
	response.CodeForbidden: {
		Status: http.StatusForbidden,
		Code:   response.CodeForbidden,
	},
	response.CodeInternalError: {
		Status: http.StatusInternalServerError,
		Code:   response.CodeInternalError,
	},
}

var OrganizationSwitchPostErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeUnauthorized: {
		Status: http.StatusUnauthorized,
		Code:   response.CodeUnauthorized,
	},
	response.CodeInvalidRequest: {
		Status: http.StatusBadRequest,
		Code:   response.CodeInvalidRequest,
	},
	response.CodeForbidden: {
		Status: http.StatusForbidden,
		Code:   response.CodeForbidden,
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

var CSRFGetErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeInternalError: {
		Status: http.StatusInternalServerError,
		Code:   response.CodeInternalError,
	},
}

var LoginPostErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeInvalidRequest: {
		Status: http.StatusBadRequest,
		Code:   response.CodeInvalidRequest,
	},
	response.CodeUnauthorized: {
		Status: http.StatusUnauthorized,
		Code:   response.CodeUnauthorized,
	},
	response.CodeForbidden: {
		Status: http.StatusForbidden,
		Code:   response.CodeForbidden,
	},
	response.CodeRateLimited: {
		Status: http.StatusTooManyRequests,
		Code:   response.CodeRateLimited,
	},
	response.CodeInternalError: {
		Status: http.StatusInternalServerError,
		Code:   response.CodeInternalError,
	},
}

var SignupPostErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeInvalidRequest: {
		Status: http.StatusBadRequest,
		Code:   response.CodeInvalidRequest,
	},
	response.CodeForbidden: {
		Status: http.StatusForbidden,
		Code:   response.CodeForbidden,
	},
	response.CodeRateLimited: {
		Status: http.StatusTooManyRequests,
		Code:   response.CodeRateLimited,
	},
	response.CodeInternalError: {
		Status: http.StatusInternalServerError,
		Code:   response.CodeInternalError,
	},
}

var SignupVerifyPostErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeInvalidRequest: {
		Status: http.StatusBadRequest,
		Code:   response.CodeInvalidRequest,
	},
	response.CodeForbidden: {
		Status: http.StatusForbidden,
		Code:   response.CodeForbidden,
	},
	response.CodeNotFound: {
		Status: http.StatusNotFound,
		Code:   response.CodeNotFound,
	},
	response.CodeRateLimited: {
		Status: http.StatusTooManyRequests,
		Code:   response.CodeRateLimited,
	},
	response.CodeInternalError: {
		Status: http.StatusInternalServerError,
		Code:   response.CodeInternalError,
	},
}

var ChallengeResendPostErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeInvalidRequest: {
		Status: http.StatusBadRequest,
		Code:   response.CodeInvalidRequest,
	},
	response.CodeForbidden: {
		Status: http.StatusForbidden,
		Code:   response.CodeForbidden,
	},
	response.CodeNotFound: {
		Status: http.StatusNotFound,
		Code:   response.CodeNotFound,
	},
	response.CodeRateLimited: {
		Status: http.StatusTooManyRequests,
		Code:   response.CodeRateLimited,
	},
	response.CodeInternalError: {
		Status: http.StatusInternalServerError,
		Code:   response.CodeInternalError,
	},
}

var PasskeyAuthenticateOptionsPostErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeForbidden: {
		Status: http.StatusForbidden,
		Code:   response.CodeForbidden,
	},
	response.CodeRateLimited: {
		Status: http.StatusTooManyRequests,
		Code:   response.CodeRateLimited,
	},
	response.CodeInternalError: {
		Status: http.StatusInternalServerError,
		Code:   response.CodeInternalError,
	},
}

var PasskeyAuthenticateFinishPostErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeInvalidRequest: {
		Status: http.StatusBadRequest,
		Code:   response.CodeInvalidRequest,
	},
	response.CodeUnauthorized: {
		Status: http.StatusUnauthorized,
		Code:   response.CodeUnauthorized,
	},
	response.CodeForbidden: {
		Status: http.StatusForbidden,
		Code:   response.CodeForbidden,
	},
	response.CodeRateLimited: {
		Status: http.StatusTooManyRequests,
		Code:   response.CodeRateLimited,
	},
	response.CodeInternalError: {
		Status: http.StatusInternalServerError,
		Code:   response.CodeInternalError,
	},
}

var PasskeyRegisterOptionsPostErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeUnauthorized: {
		Status: http.StatusUnauthorized,
		Code:   response.CodeUnauthorized,
	},
	response.CodeForbidden: {
		Status: http.StatusForbidden,
		Code:   response.CodeForbidden,
	},
	response.CodeInternalError: {
		Status: http.StatusInternalServerError,
		Code:   response.CodeInternalError,
	},
}

var PasskeyRegisterFinishPostErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeInvalidRequest: {
		Status: http.StatusBadRequest,
		Code:   response.CodeInvalidRequest,
	},
	response.CodeUnauthorized: {
		Status: http.StatusUnauthorized,
		Code:   response.CodeUnauthorized,
	},
	response.CodeForbidden: {
		Status: http.StatusForbidden,
		Code:   response.CodeForbidden,
	},
	codePasskeyAlreadyExists: {
		Status: http.StatusConflict,
		Code:   codePasskeyAlreadyExists,
	},
	codePasskeyRegistrationInvalid: {
		Status: http.StatusUnprocessableEntity,
		Code:   codePasskeyRegistrationInvalid,
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
		Path:   "/auth/api/v1/csrf",
		Errors: CSRFGetErrors,
	},
	{
		Method: http.MethodGet,
		Path:   "/auth/api/v1/oauth/google/options",
		Errors: GoogleOptionsGetErrors,
	},
	{
		Method: http.MethodPost,
		Path:   "/auth/api/v1/oauth/google",
		Errors: GoogleLoginPostErrors,
	},
	{
		Method: http.MethodPost,
		Path:   "/auth/api/v1/login",
		Errors: LoginPostErrors,
	},
	{
		Method: http.MethodPost,
		Path:   "/auth/api/v1/signup",
		Errors: SignupPostErrors,
	},
	{
		Method: http.MethodPost,
		Path:   "/auth/api/v1/signup/verify",
		Errors: SignupVerifyPostErrors,
	},
	{
		Method: http.MethodPost,
		Path:   "/auth/api/v1/challenges/resend",
		Errors: ChallengeResendPostErrors,
	},
	{
		Method: http.MethodPost,
		Path:   "/auth/api/v1/passkeys/authenticate/options",
		Errors: PasskeyAuthenticateOptionsPostErrors,
	},
	{
		Method: http.MethodPost,
		Path:   "/auth/api/v1/passkeys/authenticate/finish",
		Errors: PasskeyAuthenticateFinishPostErrors,
	},
	{
		Method: http.MethodPost,
		Path:   "/auth/api/v1/passkeys/register/options",
		Errors: PasskeyRegisterOptionsPostErrors,
	},
	{
		Method: http.MethodPost,
		Path:   "/auth/api/v1/passkeys/register/finish",
		Errors: PasskeyRegisterFinishPostErrors,
	},
	{
		Method: http.MethodGet,
		Path:   "/auth/api/v1/user",
		Errors: UserGetErrors,
	},
	{
		Method: http.MethodGet,
		Path:   "/auth/api/v1/organizations",
		Errors: OrganizationsGetErrors,
	},
	{
		Method: http.MethodGet,
		Path:   "/auth/api/v1/organizations/current",
		Errors: OrganizationsGetErrors,
	},
	{
		Method: http.MethodGet,
		Path:   "/auth/api/v1/organizations/current/members",
		Errors: OrganizationMembersGetErrors,
	},
	{
		Method: http.MethodPost,
		Path:   "/auth/api/v1/organizations/{organizationID}/switch",
		Errors: OrganizationSwitchPostErrors,
	},
	{
		Method: http.MethodPost,
		Path:   "/auth/api/v1/sessions/refresh",
		Errors: RefreshPostErrors,
	},
	{
		Method: http.MethodPost,
		Path:   "/auth/api/v1/tokens/refresh",
		Errors: RefreshPostErrors,
	},
	{
		Method: http.MethodPost,
		Path:   "/auth/api/v1/sessions/logout",
		Errors: LogoutPostErrors,
	},
}
