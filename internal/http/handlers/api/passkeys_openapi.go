package api

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/passkey"
	"github.com/authara-org/authara/internal/session/token"
)

func (h *APIHandler) BeginPasskeyAuthentication(ctx context.Context, _ contract.BeginPasskeyAuthenticationRequestObject) (contract.BeginPasskeyAuthenticationResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return beginPasskeyAuthenticationError(responseCodeInternalError(), "API contract error."), nil
	}
	if h.Passkeys == nil {
		return beginPasskeyAuthenticationError(responseCodeInternalError(), "Passkey error."), nil
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowPasskeyLoginAttempt(ctx, httputil.ClientIP(r))
		if err != nil || !allowed {
			return beginPasskeyAuthenticationError(responseCodeRateLimited(), "Too many attempts. Please try again later."), nil
		}
	}
	optionsJSON, _, err := h.Passkeys.BeginLogin(ctx)
	if err != nil {
		return beginPasskeyAuthenticationError(responseCodeInternalError(), "Passkey error."), nil
	}
	out, code, message, ok := passkeyOptionsResponse(optionsJSON)
	if !ok {
		return beginPasskeyAuthenticationError(code, message), nil
	}
	return contract.BeginPasskeyAuthentication200JSONResponse(out), nil
}

func (h *APIHandler) FinishPasskeyAuthentication(ctx context.Context, request contract.FinishPasskeyAuthenticationRequestObject) (contract.FinishPasskeyAuthenticationResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return finishPasskeyAuthenticationError(responseCodeInternalError(), "API contract error."), nil
	}
	if h.Passkeys == nil {
		return finishPasskeyAuthenticationError(responseCodeInternalError(), "Passkey error."), nil
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowPasskeyLoginFinishAttempt(ctx, httputil.ClientIP(r))
		if err != nil || !allowed {
			return finishPasskeyAuthenticationError(responseCodeRateLimited(), "Too many attempts. Please try again later."), nil
		}
	}
	if request.Body == nil {
		return finishPasskeyAuthenticationError(responseCodeInvalidRequest(), "Invalid passkey response."), nil
	}
	credential, err := json.Marshal(request.Body.Credential)
	if err != nil {
		return finishPasskeyAuthenticationError(responseCodeInvalidRequest(), "Invalid passkey response."), nil
	}
	audience := token.AudienceApp
	if request.Params.Audience != nil {
		audience = token.Audience(*request.Params.Audience)
	}
	user, err := h.Passkeys.FinishLogin(ctx, request.Body.ChallengeId, credential, time.Now().UTC())
	if errors.Is(err, passkey.ErrPasskeyAuthenticationInvalid) {
		return finishPasskeyAuthenticationError(responseCodeUnauthorized(), "Passkey sign-in failed."), nil
	}
	if err != nil {
		return finishPasskeyAuthenticationError(responseCodeInternalError(), "Passkey error."), nil
	}
	body, header, code, message, ok := h.contractSession(ctx, r, user, audience)
	if !ok {
		return finishPasskeyAuthenticationError(code, message), nil
	}
	return contract.FinishPasskeyAuthentication200HeadersResponse{Header: header, Body: body}, nil
}

func (h *APIHandler) BeginPasskeyRegistration(ctx context.Context, _ contract.BeginPasskeyRegistrationRequestObject) (contract.BeginPasskeyRegistrationResponseObject, error) {
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return beginPasskeyRegistrationError(responseCodeUnauthorized(), "Unauthorized."), nil
	}
	if h.Passkeys == nil {
		return beginPasskeyRegistrationError(responseCodeInternalError(), "Passkey error."), nil
	}
	optionsJSON, _, err := h.Passkeys.BeginRegistration(ctx, userID)
	if err != nil {
		return beginPasskeyRegistrationError(responseCodeInternalError(), "Passkey error."), nil
	}
	out, code, message, ok := passkeyOptionsResponse(optionsJSON)
	if !ok {
		return beginPasskeyRegistrationError(code, message), nil
	}
	return contract.BeginPasskeyRegistration200JSONResponse(out), nil
}

func (h *APIHandler) FinishPasskeyRegistration(ctx context.Context, request contract.FinishPasskeyRegistrationRequestObject) (contract.FinishPasskeyRegistrationResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return finishPasskeyRegistrationError(responseCodeInternalError(), "API contract error."), nil
	}
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return finishPasskeyRegistrationError(responseCodeUnauthorized(), "Unauthorized."), nil
	}
	if h.Passkeys == nil {
		return finishPasskeyRegistrationError(responseCodeInternalError(), "Passkey error."), nil
	}
	if request.Body == nil {
		return finishPasskeyRegistrationError(responseCodeInvalidRequest(), "Invalid passkey response."), nil
	}
	credential, err := json.Marshal(request.Body.Credential)
	if err != nil {
		return finishPasskeyRegistrationError(responseCodeInvalidRequest(), "Invalid passkey response."), nil
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
		return finishPasskeyRegistrationError(codePasskeyAlreadyExists, "This passkey is already linked to an account."), nil
	case errors.Is(err, passkey.ErrPasskeyRegistrationInvalid):
		return finishPasskeyRegistrationError(codePasskeyRegistrationInvalid, "Passkey setup could not be verified."), nil
	case err != nil:
		return finishPasskeyRegistrationError(responseCodeInternalError(), "Passkey error."), nil
	}
	return contract.FinishPasskeyRegistration204Response{}, nil
}

func passkeyOptionsResponse(body []byte) (contract.PasskeyOptions, response.ErrorCode, string, bool) {
	var out contract.PasskeyOptions
	if err := json.Unmarshal(body, &out); err != nil {
		return contract.PasskeyOptions{}, response.CodeInternalError, "Passkey error.", false
	}
	return out, "", "", true
}
