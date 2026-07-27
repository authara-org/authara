package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/passkey"
	"github.com/authara-org/authara/internal/session/token"
)

func (h *APIHandler) BeginPasskeyAuthentication(ctx context.Context, _ contract.BeginPasskeyAuthenticationRequestObject) (contract.BeginPasskeyAuthenticationResponseObject, error) {
	r, out, ok := contractRequest(ctx)
	if !ok {
		return out, nil
	}
	if h.Passkeys == nil {
		return routeError(BeginPasskeyAuthenticationErrors, responseCodeInternalError(), "Passkey error."), nil
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowPasskeyLoginAttempt(ctx, httputil.ClientIP(r))
		if err != nil || !allowed {
			return routeError(BeginPasskeyAuthenticationErrors, responseCodeRateLimited(), "Too many attempts. Please try again later."), nil
		}
	}
	optionsJSON, _, err := h.Passkeys.BeginLogin(ctx)
	if err != nil {
		return routeError(BeginPasskeyAuthenticationErrors, responseCodeInternalError(), "Passkey error."), nil
	}
	return passkeyOptionsResponse(BeginPasskeyAuthenticationErrors, optionsJSON), nil
}

func (h *APIHandler) FinishPasskeyAuthentication(ctx context.Context, request contract.FinishPasskeyAuthenticationRequestObject) (contract.FinishPasskeyAuthenticationResponseObject, error) {
	r, out, ok := contractRequest(ctx)
	if !ok {
		return out, nil
	}
	if h.Passkeys == nil {
		return routeError(FinishPasskeyAuthenticationErrors, responseCodeInternalError(), "Passkey error."), nil
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowPasskeyLoginFinishAttempt(ctx, httputil.ClientIP(r))
		if err != nil || !allowed {
			return routeError(FinishPasskeyAuthenticationErrors, responseCodeRateLimited(), "Too many attempts. Please try again later."), nil
		}
	}
	if request.Body == nil {
		return invalidPasskeyResponse(FinishPasskeyAuthenticationErrors), nil
	}
	credential, err := json.Marshal(request.Body.Credential)
	if err != nil {
		return invalidPasskeyResponse(FinishPasskeyAuthenticationErrors), nil
	}
	audience := token.AudienceApp
	if request.Params.Audience != nil {
		audience = token.Audience(*request.Params.Audience)
	}
	user, err := h.Passkeys.FinishLogin(ctx, request.Body.ChallengeId, credential, time.Now().UTC())
	if errors.Is(err, passkey.ErrPasskeyAuthenticationInvalid) {
		return routeError(FinishPasskeyAuthenticationErrors, responseCodeUnauthorized(), "Passkey sign-in failed."), nil
	}
	if err != nil {
		return routeError(FinishPasskeyAuthenticationErrors, responseCodeInternalError(), "Passkey error."), nil
	}
	return h.contractSessionResponse(ctx, r, FinishPasskeyAuthenticationErrors, user, audience, http.StatusOK), nil
}

func (h *APIHandler) BeginPasskeyRegistration(ctx context.Context, _ contract.BeginPasskeyRegistrationRequestObject) (contract.BeginPasskeyRegistrationResponseObject, error) {
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return routeError(BeginPasskeyRegistrationErrors, responseCodeUnauthorized(), "Unauthorized."), nil
	}
	if h.Passkeys == nil {
		return routeError(BeginPasskeyRegistrationErrors, responseCodeInternalError(), "Passkey error."), nil
	}
	optionsJSON, _, err := h.Passkeys.BeginRegistration(ctx, userID)
	if err != nil {
		return routeError(BeginPasskeyRegistrationErrors, responseCodeInternalError(), "Passkey error."), nil
	}
	return passkeyOptionsResponse(BeginPasskeyRegistrationErrors, optionsJSON), nil
}

func (h *APIHandler) FinishPasskeyRegistration(ctx context.Context, request contract.FinishPasskeyRegistrationRequestObject) (contract.FinishPasskeyRegistrationResponseObject, error) {
	r, out, ok := contractRequest(ctx)
	if !ok {
		return out, nil
	}
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return routeError(FinishPasskeyRegistrationErrors, responseCodeUnauthorized(), "Unauthorized."), nil
	}
	if h.Passkeys == nil {
		return routeError(FinishPasskeyRegistrationErrors, responseCodeInternalError(), "Passkey error."), nil
	}
	if request.Body == nil {
		return invalidPasskeyResponse(FinishPasskeyRegistrationErrors), nil
	}
	credential, err := json.Marshal(request.Body.Credential)
	if err != nil {
		return invalidPasskeyResponse(FinishPasskeyRegistrationErrors), nil
	}
	name := ""
	if request.Body.Name != nil {
		name = *request.Body.Name
	}
	platformHint := ""
	if request.Body.PlatformHint != nil {
		platformHint = *request.Body.PlatformHint
	}
	err = h.Passkeys.FinishRegistration(ctx, userID, request.Body.ChallengeId, credential, passkey.RegistrationMetadata{
		Name:         name,
		UserAgent:    r.UserAgent(),
		PlatformHint: platformHint,
	})
	switch {
	case errors.Is(err, passkey.ErrPasskeyAlreadyExists):
		return routeError(FinishPasskeyRegistrationErrors, codePasskeyAlreadyExists, "This passkey is already linked to an account."), nil
	case errors.Is(err, passkey.ErrPasskeyRegistrationInvalid):
		return routeError(FinishPasskeyRegistrationErrors, codePasskeyRegistrationInvalid, "Passkey setup could not be verified."), nil
	case err != nil:
		return routeError(FinishPasskeyRegistrationErrors, responseCodeInternalError(), "Passkey error."), nil
	}
	return contract.NoContent(), nil
}

func passkeyOptionsResponse(routeErrors map[response.ErrorCode]response.ErrorSpec, body []byte) contract.Response {
	var out contract.PasskeyOptions
	if err := json.Unmarshal(body, &out); err != nil {
		return routeError(routeErrors, response.CodeInternalError, "Passkey error.")
	}
	return contract.JSON(http.StatusOK, out)
}

func invalidPasskeyResponse(routeErrors map[response.ErrorCode]response.ErrorSpec) contract.Response {
	return routeError(routeErrors, response.CodeInvalidRequest, "Invalid passkey response.")
}
