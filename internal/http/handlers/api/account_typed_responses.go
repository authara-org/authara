package api

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
)

func getCurrentAccountError(code response.ErrorCode, message string) contract.GetCurrentAccountResponseObject {
	spec := mustRouteError(GetCurrentAccountErrors, code)
	body := apiErrorBody(spec.Code, message)
	if spec.Status == http.StatusUnauthorized {
		return contract.GetCurrentAccount401JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	}
	return contract.GetCurrentAccount500JSONResponse(body)
}

func changeCurrentUsernameError(code response.ErrorCode, message string) contract.ChangeCurrentUsernameResponseObject {
	spec := mustRouteError(ChangeCurrentUsernameErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.ChangeCurrentUsername400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.ChangeCurrentUsername401JSONResponse(body)
	case http.StatusConflict:
		return contract.ChangeCurrentUsername409JSONResponse(body)
	default:
		return contract.ChangeCurrentUsername500JSONResponse(body)
	}
}

func startCurrentUserEmailChangeError(code response.ErrorCode, message string) contract.StartCurrentUserEmailChangeResponseObject {
	spec := mustRouteError(StartCurrentUserEmailChangeErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.StartCurrentUserEmailChange400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.StartCurrentUserEmailChange401JSONResponse(body)
	case http.StatusForbidden:
		return contract.StartCurrentUserEmailChange403JSONResponse(body)
	case http.StatusNotFound:
		return contract.StartCurrentUserEmailChange404JSONResponse(body)
	default:
		return contract.StartCurrentUserEmailChange500JSONResponse(body)
	}
}

func verifyCurrentUserEmailChangeError(code response.ErrorCode, message string) contract.VerifyCurrentUserEmailChangeResponseObject {
	spec := mustRouteError(VerifyCurrentUserEmailChangeErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.VerifyCurrentUserEmailChange400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.VerifyCurrentUserEmailChange401JSONResponse(body)
	case http.StatusForbidden:
		return contract.VerifyCurrentUserEmailChange403JSONResponse(body)
	case http.StatusNotFound:
		return contract.VerifyCurrentUserEmailChange404JSONResponse(body)
	case http.StatusTooManyRequests:
		return contract.VerifyCurrentUserEmailChange429JSONResponse(body)
	default:
		return contract.VerifyCurrentUserEmailChange500JSONResponse(body)
	}
}

func changeCurrentUserPasswordError(code response.ErrorCode, message string) contract.ChangeCurrentUserPasswordResponseObject {
	spec := mustRouteError(ChangeCurrentUserPasswordErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.ChangeCurrentUserPassword400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.ChangeCurrentUserPassword401JSONResponse(body)
	case http.StatusNotFound:
		return contract.ChangeCurrentUserPassword404JSONResponse(body)
	default:
		return contract.ChangeCurrentUserPassword500JSONResponse(body)
	}
}

func addCurrentUserPasswordError(code response.ErrorCode, message string) contract.AddCurrentUserPasswordResponseObject {
	spec := mustRouteError(AddCurrentUserPasswordErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.AddCurrentUserPassword400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.AddCurrentUserPassword401JSONResponse(body)
	case http.StatusConflict:
		return contract.AddCurrentUserPassword409JSONResponse(body)
	default:
		return contract.AddCurrentUserPassword500JSONResponse(body)
	}
}

func linkCurrentUserGoogleError(code response.ErrorCode, message string) contract.LinkCurrentUserGoogleResponseObject {
	spec := mustRouteError(LinkCurrentUserGoogleErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.LinkCurrentUserGoogle400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.LinkCurrentUserGoogle401JSONResponse(body)
	case http.StatusForbidden:
		return contract.LinkCurrentUserGoogle403JSONResponse(body)
	case http.StatusNotFound:
		return contract.LinkCurrentUserGoogle404JSONResponse(body)
	case http.StatusConflict:
		return contract.LinkCurrentUserGoogle409JSONResponse(body)
	default:
		return contract.LinkCurrentUserGoogle500JSONResponse(body)
	}
}

func unlinkCurrentUserAuthMethodError(code response.ErrorCode, message string) contract.UnlinkCurrentUserAuthMethodResponseObject {
	spec := mustRouteError(UnlinkCurrentUserAuthMethodErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.UnlinkCurrentUserAuthMethod400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.UnlinkCurrentUserAuthMethod401JSONResponse(body)
	case http.StatusNotFound:
		return contract.UnlinkCurrentUserAuthMethod404JSONResponse(body)
	case http.StatusConflict:
		return contract.UnlinkCurrentUserAuthMethod409JSONResponse(body)
	default:
		return contract.UnlinkCurrentUserAuthMethod500JSONResponse(body)
	}
}

func deleteCurrentUserPasskeyError(code response.ErrorCode, message string) contract.DeleteCurrentUserPasskeyResponseObject {
	spec := mustRouteError(DeleteCurrentUserPasskeyErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.DeleteCurrentUserPasskey400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.DeleteCurrentUserPasskey401JSONResponse(body)
	case http.StatusNotFound:
		return contract.DeleteCurrentUserPasskey404JSONResponse(body)
	case http.StatusConflict:
		return contract.DeleteCurrentUserPasskey409JSONResponse(body)
	default:
		return contract.DeleteCurrentUserPasskey500JSONResponse(body)
	}
}

func revokeCurrentUserSessionError(code response.ErrorCode, message string) contract.RevokeCurrentUserSessionResponseObject {
	spec := mustRouteError(RevokeCurrentUserSessionErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.RevokeCurrentUserSession400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.RevokeCurrentUserSession401JSONResponse(body)
	case http.StatusForbidden:
		return contract.RevokeCurrentUserSession403JSONResponse(body)
	case http.StatusNotFound:
		return contract.RevokeCurrentUserSession404JSONResponse(body)
	default:
		return contract.RevokeCurrentUserSession500JSONResponse(body)
	}
}

func revokeCurrentUserOtherSessionsError(code response.ErrorCode, message string) contract.RevokeCurrentUserOtherSessionsResponseObject {
	spec := mustRouteError(RevokeCurrentUserOtherSessionsErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusUnauthorized:
		return contract.RevokeCurrentUserOtherSessions401JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusForbidden:
		return contract.RevokeCurrentUserOtherSessions403JSONResponse(body)
	default:
		return contract.RevokeCurrentUserOtherSessions500JSONResponse(body)
	}
}
