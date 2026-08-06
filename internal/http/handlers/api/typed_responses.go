package api

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
)

func apiErrorBody(code response.ErrorCode, message string) contract.ErrorResponse {
	return contract.ErrorResponse{
		Error: contract.APIError{
			Code:    string(code),
			Message: message,
		},
	}
}

func getCsrfTokenError(code response.ErrorCode, message string) contract.GetCsrfTokenResponseObject {
	spec := mustRouteError(GetCsrfTokenErrors, code)
	body := apiErrorBody(spec.Code, message)
	return contract.GetCsrfToken500JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
}

func getGoogleLoginOptionsError(code response.ErrorCode, message string) contract.GetGoogleLoginOptionsResponseObject {
	spec := mustRouteError(GetGoogleLoginOptionsErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusNotFound:
		return contract.GetGoogleLoginOptions404JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	default:
		return contract.GetGoogleLoginOptions500JSONResponse(body)
	}
}

func loginWithGoogleError(code response.ErrorCode, message string) contract.LoginWithGoogleResponseObject {
	spec := mustRouteError(LoginWithGoogleErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.LoginWithGoogle400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.LoginWithGoogle401JSONResponse(body)
	case http.StatusForbidden:
		return contract.LoginWithGoogle403JSONResponse(body)
	case http.StatusNotFound:
		return contract.LoginWithGoogle404JSONResponse(body)
	case http.StatusConflict:
		return contract.LoginWithGoogle409JSONResponse(body)
	default:
		return contract.LoginWithGoogle500JSONResponse(body)
	}
}

func loginWithPasswordError(code response.ErrorCode, message string) contract.LoginWithPasswordResponseObject {
	spec := mustRouteError(LoginWithPasswordErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.LoginWithPassword400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.LoginWithPassword401JSONResponse(body)
	case http.StatusForbidden:
		return contract.LoginWithPassword403JSONResponse(body)
	case http.StatusTooManyRequests:
		return contract.LoginWithPassword429JSONResponse(body)
	default:
		return contract.LoginWithPassword500JSONResponse(body)
	}
}

func signupDirectError(code response.ErrorCode, message string) contract.SignupDirectResponseObject {
	spec := mustRouteError(SignupDirectErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.SignupDirect400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusForbidden:
		return contract.SignupDirect403JSONResponse(body)
	case http.StatusNotFound:
		return contract.SignupDirect404JSONResponse(body)
	case http.StatusTooManyRequests:
		return contract.SignupDirect429JSONResponse(body)
	default:
		return contract.SignupDirect500JSONResponse(body)
	}
}

func startSignupChallengeError(code response.ErrorCode, message string) contract.StartSignupChallengeResponseObject {
	spec := mustRouteError(StartSignupChallengeErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.StartSignupChallenge400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusForbidden:
		return contract.StartSignupChallenge403JSONResponse(body)
	case http.StatusNotFound:
		return contract.StartSignupChallenge404JSONResponse(body)
	case http.StatusTooManyRequests:
		return contract.StartSignupChallenge429JSONResponse(body)
	default:
		return contract.StartSignupChallenge500JSONResponse(body)
	}
}

func verifySignupChallengeError(code response.ErrorCode, message string) contract.VerifySignupChallengeResponseObject {
	spec := mustRouteError(VerifySignupChallengeErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.VerifySignupChallenge400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusForbidden:
		return contract.VerifySignupChallenge403JSONResponse(body)
	case http.StatusNotFound:
		return contract.VerifySignupChallenge404JSONResponse(body)
	case http.StatusTooManyRequests:
		return contract.VerifySignupChallenge429JSONResponse(body)
	default:
		return contract.VerifySignupChallenge500JSONResponse(body)
	}
}

func startPasswordResetChallengeError(code response.ErrorCode, message string) contract.StartPasswordResetChallengeResponseObject {
	spec := mustRouteError(StartPasswordResetChallengeErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.StartPasswordResetChallenge400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusForbidden:
		return contract.StartPasswordResetChallenge403JSONResponse(body)
	case http.StatusTooManyRequests:
		return contract.StartPasswordResetChallenge429JSONResponse(body)
	default:
		return contract.StartPasswordResetChallenge500JSONResponse(body)
	}
}

func verifyPasswordResetChallengeError(code response.ErrorCode, message string) contract.VerifyPasswordResetChallengeResponseObject {
	spec := mustRouteError(VerifyPasswordResetChallengeErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.VerifyPasswordResetChallenge400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusForbidden:
		return contract.VerifyPasswordResetChallenge403JSONResponse(body)
	case http.StatusTooManyRequests:
		return contract.VerifyPasswordResetChallenge429JSONResponse(body)
	default:
		return contract.VerifyPasswordResetChallenge500JSONResponse(body)
	}
}

func resendChallengeError(code response.ErrorCode, message string) contract.ResendChallengeResponseObject {
	spec := mustRouteError(ResendChallengeErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.ResendChallenge400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusForbidden:
		return contract.ResendChallenge403JSONResponse(body)
	case http.StatusTooManyRequests:
		return contract.ResendChallenge429JSONResponse(body)
	default:
		return contract.ResendChallenge500JSONResponse(body)
	}
}

func beginPasskeyAuthenticationError(code response.ErrorCode, message string) contract.BeginPasskeyAuthenticationResponseObject {
	spec := mustRouteError(BeginPasskeyAuthenticationErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusForbidden:
		return contract.BeginPasskeyAuthentication403JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusTooManyRequests:
		return contract.BeginPasskeyAuthentication429JSONResponse(body)
	default:
		return contract.BeginPasskeyAuthentication500JSONResponse(body)
	}
}

func finishPasskeyAuthenticationError(code response.ErrorCode, message string) contract.FinishPasskeyAuthenticationResponseObject {
	spec := mustRouteError(FinishPasskeyAuthenticationErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.FinishPasskeyAuthentication400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.FinishPasskeyAuthentication401JSONResponse(body)
	case http.StatusForbidden:
		return contract.FinishPasskeyAuthentication403JSONResponse(body)
	case http.StatusTooManyRequests:
		return contract.FinishPasskeyAuthentication429JSONResponse(body)
	default:
		return contract.FinishPasskeyAuthentication500JSONResponse(body)
	}
}

func beginPasskeyRegistrationError(code response.ErrorCode, message string) contract.BeginPasskeyRegistrationResponseObject {
	spec := mustRouteError(BeginPasskeyRegistrationErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusUnauthorized:
		return contract.BeginPasskeyRegistration401JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusForbidden:
		return contract.BeginPasskeyRegistration403JSONResponse(body)
	default:
		return contract.BeginPasskeyRegistration500JSONResponse(body)
	}
}

func finishPasskeyRegistrationError(code response.ErrorCode, message string) contract.FinishPasskeyRegistrationResponseObject {
	spec := mustRouteError(FinishPasskeyRegistrationErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.FinishPasskeyRegistration400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.FinishPasskeyRegistration401JSONResponse(body)
	case http.StatusForbidden:
		return contract.FinishPasskeyRegistration403JSONResponse(body)
	case http.StatusConflict:
		return contract.FinishPasskeyRegistration409JSONResponse(body)
	case http.StatusUnprocessableEntity:
		return contract.FinishPasskeyRegistration422JSONResponse(body)
	default:
		return contract.FinishPasskeyRegistration500JSONResponse(body)
	}
}

func logoutError(code response.ErrorCode, message string) contract.LogoutResponseObject {
	spec := mustRouteError(LogoutErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusForbidden:
		return contract.Logout403JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	default:
		return contract.Logout500JSONResponse(body)
	}
}

func refreshSessionError(code response.ErrorCode, message string, header http.Header) contract.RefreshSessionResponseObject {
	spec := mustRouteError(RefreshSessionErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.RefreshSession400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		if len(header) > 0 {
			return contract.RefreshSession401HeadersResponse{Header: header, Body: body}
		}
		return contract.RefreshSession401JSONResponse(body)
	default:
		return contract.RefreshSession500JSONResponse(body)
	}
}

func refreshTokensError(code response.ErrorCode, message string) contract.RefreshTokensResponseObject {
	spec := mustRouteError(RefreshTokensErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.RefreshTokens400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.RefreshTokens401JSONResponse(body)
	default:
		return contract.RefreshTokens500JSONResponse(body)
	}
}

func getCurrentUserError(code response.ErrorCode, message string) contract.GetCurrentUserResponseObject {
	spec := mustRouteError(GetCurrentUserErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusUnauthorized:
		return contract.GetCurrentUser401JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	default:
		return contract.GetCurrentUser500JSONResponse(body)
	}
}

func setCurrentUserPasswordError(code response.ErrorCode, message string) contract.SetCurrentUserPasswordResponseObject {
	spec := mustRouteError(SetCurrentUserPasswordErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.SetCurrentUserPassword400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.SetCurrentUserPassword401JSONResponse(body)
	case http.StatusForbidden:
		return contract.SetCurrentUserPassword403JSONResponse(body)
	default:
		return contract.SetCurrentUserPassword500JSONResponse(body)
	}
}

func listCurrentUserOrganizationsError(code response.ErrorCode, message string) contract.ListCurrentUserOrganizationsResponseObject {
	spec := mustRouteError(ListCurrentUserOrganizationsErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusUnauthorized:
		return contract.ListCurrentUserOrganizations401JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	default:
		return contract.ListCurrentUserOrganizations500JSONResponse(body)
	}
}

func getCurrentOrganizationError(code response.ErrorCode, message string) contract.GetCurrentOrganizationResponseObject {
	spec := mustRouteError(GetCurrentOrganizationErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusUnauthorized:
		return contract.GetCurrentOrganization401JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	default:
		return contract.GetCurrentOrganization500JSONResponse(body)
	}
}

func listCurrentOrganizationMembersError(code response.ErrorCode, message string) contract.ListCurrentOrganizationMembersResponseObject {
	spec := mustRouteError(ListCurrentOrganizationMembersErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusUnauthorized:
		return contract.ListCurrentOrganizationMembers401JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusForbidden:
		return contract.ListCurrentOrganizationMembers403JSONResponse(body)
	default:
		return contract.ListCurrentOrganizationMembers500JSONResponse(body)
	}
}

func switchOrganizationError(code response.ErrorCode, message string) contract.SwitchOrganizationResponseObject {
	spec := mustRouteError(SwitchOrganizationErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.SwitchOrganization400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.SwitchOrganization401JSONResponse(body)
	case http.StatusForbidden:
		return contract.SwitchOrganization403JSONResponse(body)
	default:
		return contract.SwitchOrganization500JSONResponse(body)
	}
}
