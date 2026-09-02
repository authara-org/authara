package api

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
)

func previewInvitationError(code response.ErrorCode, message string) contract.PreviewInvitationResponseObject {
	spec := mustRouteError(PreviewInvitationErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.PreviewInvitation400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusForbidden:
		return contract.PreviewInvitation403JSONResponse(body)
	case http.StatusNotFound:
		return contract.PreviewInvitation404JSONResponse(body)
	default:
		return contract.PreviewInvitation500JSONResponse(body)
	}
}

func acceptInvitationError(code response.ErrorCode, message string) contract.AcceptInvitationResponseObject {
	spec := mustRouteError(AcceptInvitationErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.AcceptInvitation400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.AcceptInvitation401JSONResponse(body)
	case http.StatusForbidden:
		return contract.AcceptInvitation403JSONResponse(body)
	case http.StatusNotFound:
		return contract.AcceptInvitation404JSONResponse(body)
	case http.StatusConflict:
		return contract.AcceptInvitation409JSONResponse(body)
	default:
		return contract.AcceptInvitation500JSONResponse(body)
	}
}

func loginAndAcceptInvitationError(code response.ErrorCode, message string) contract.LoginAndAcceptInvitationResponseObject {
	spec := mustRouteError(LoginAndAcceptInvitationErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.LoginAndAcceptInvitation400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.LoginAndAcceptInvitation401JSONResponse(body)
	case http.StatusForbidden:
		return contract.LoginAndAcceptInvitation403JSONResponse(body)
	case http.StatusNotFound:
		return contract.LoginAndAcceptInvitation404JSONResponse(body)
	case http.StatusConflict:
		return contract.LoginAndAcceptInvitation409JSONResponse(body)
	case http.StatusTooManyRequests:
		return contract.LoginAndAcceptInvitation429JSONResponse(body)
	default:
		return contract.LoginAndAcceptInvitation500JSONResponse(body)
	}
}

func authenticateAndAcceptInvitationWithGoogleError(code response.ErrorCode, message string) contract.AuthenticateAndAcceptInvitationWithGoogleResponseObject {
	spec := mustRouteError(AuthenticateAndAcceptInvitationWithGoogleErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.AuthenticateAndAcceptInvitationWithGoogle400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.AuthenticateAndAcceptInvitationWithGoogle401JSONResponse(body)
	case http.StatusForbidden:
		return contract.AuthenticateAndAcceptInvitationWithGoogle403JSONResponse(body)
	case http.StatusNotFound:
		return contract.AuthenticateAndAcceptInvitationWithGoogle404JSONResponse(body)
	case http.StatusConflict:
		return contract.AuthenticateAndAcceptInvitationWithGoogle409JSONResponse(body)
	default:
		return contract.AuthenticateAndAcceptInvitationWithGoogle500JSONResponse(body)
	}
}

func startGoogleAccountRecoveryLinkError(code response.ErrorCode, message string) contract.StartGoogleAccountRecoveryLinkResponseObject {
	spec := mustRouteError(StartGoogleAccountRecoveryLinkErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.StartGoogleAccountRecoveryLink400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.StartGoogleAccountRecoveryLink401JSONResponse(body)
	case http.StatusForbidden:
		return contract.StartGoogleAccountRecoveryLink403JSONResponse(body)
	case http.StatusNotFound:
		return contract.StartGoogleAccountRecoveryLink404JSONResponse(body)
	case http.StatusConflict:
		return contract.StartGoogleAccountRecoveryLink409JSONResponse(body)
	default:
		return contract.StartGoogleAccountRecoveryLink500JSONResponse(body)
	}
}

func completeAccountRecoveryLinkWithPasswordError(code response.ErrorCode, message string) contract.CompleteAccountRecoveryLinkWithPasswordResponseObject {
	spec := mustRouteError(CompleteAccountRecoveryLinkWithPasswordErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.CompleteAccountRecoveryLinkWithPassword400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.CompleteAccountRecoveryLinkWithPassword401JSONResponse(body)
	case http.StatusForbidden:
		return contract.CompleteAccountRecoveryLinkWithPassword403JSONResponse(body)
	case http.StatusNotFound:
		return contract.CompleteAccountRecoveryLinkWithPassword404JSONResponse(body)
	case http.StatusConflict:
		return contract.CompleteAccountRecoveryLinkWithPassword409JSONResponse(body)
	case http.StatusTooManyRequests:
		return contract.CompleteAccountRecoveryLinkWithPassword429JSONResponse(body)
	default:
		return contract.CompleteAccountRecoveryLinkWithPassword500JSONResponse(body)
	}
}

func completeAccountRecoveryLinkWithGoogleError(code response.ErrorCode, message string) contract.CompleteAccountRecoveryLinkWithGoogleResponseObject {
	spec := mustRouteError(CompleteAccountRecoveryLinkWithGoogleErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.CompleteAccountRecoveryLinkWithGoogle400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.CompleteAccountRecoveryLinkWithGoogle401JSONResponse(body)
	case http.StatusForbidden:
		return contract.CompleteAccountRecoveryLinkWithGoogle403JSONResponse(body)
	case http.StatusNotFound:
		return contract.CompleteAccountRecoveryLinkWithGoogle404JSONResponse(body)
	case http.StatusConflict:
		return contract.CompleteAccountRecoveryLinkWithGoogle409JSONResponse(body)
	default:
		return contract.CompleteAccountRecoveryLinkWithGoogle500JSONResponse(body)
	}
}
