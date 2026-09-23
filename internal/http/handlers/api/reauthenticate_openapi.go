package api

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/passkey"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/store"
)

func (h *APIHandler) ReauthenticateWithPassword(ctx context.Context, request contract.ReauthenticateWithPasswordRequestObject) (contract.ReauthenticateWithPasswordResponseObject, error) {
	r, requestOK := contractRequest(ctx)
	if !requestOK {
		return reauthenticateWithPasswordError(response.CodeInternalError, "API contract error."), nil
	}
	userID, userOK := httpctx.UserID(ctx)
	sessionID, sessionOK := httpctx.SessionID(ctx)
	if !userOK || !sessionOK {
		return reauthenticateWithPasswordError(response.CodeUnauthorized, "Unauthorized."), nil
	}
	if request.Body == nil || strings.TrimSpace(request.Body.Password) == "" {
		return reauthenticateWithPasswordError(response.CodeInvalidRequest, "Password required."), nil
	}
	now := time.Now().UTC()
	if err := h.Session.ValidateAuthenticationChallenge(ctx, userID, sessionID, request.Body.AuthenticationChallengeId, now); err != nil {
		if errors.Is(err, session.ErrAuthenticationChallengeInvalid) {
			return reauthenticateWithPasswordError(response.CodeInvalidAuthenticationChallenge, "Authentication challenge is invalid or expired."), nil
		}
		return reauthenticateWithPasswordError(response.CodeInternalError, "Session error."), nil
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowLoginAttempt(ctx, httputil.ClientIP(r), userID.String())
		if err != nil || !allowed {
			return reauthenticateWithPasswordError(response.CodeRateLimited, "Too many attempts. Please try again later."), nil
		}
	}
	if err := h.Auth.VerifyPassword(ctx, userID, request.Body.Password); err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) || errors.Is(err, store.ErrorAuthProviderNotFound) {
			return reauthenticateWithPasswordError(response.CodeUnauthorized, "Invalid password."), nil
		}
		return reauthenticateWithPasswordError(response.CodeInternalError, "Authentication error."), nil
	}
	if err := h.Session.CompleteAuthenticationChallenge(ctx, userID, sessionID, request.Body.AuthenticationChallengeId, domain.AuthenticationMethodPassword, now); err != nil {
		if errors.Is(err, session.ErrAuthenticationChallengeInvalid) {
			return reauthenticateWithPasswordError(response.CodeInvalidAuthenticationChallenge, "Authentication challenge is invalid or expired."), nil
		}
		return reauthenticateWithPasswordError(response.CodeInternalError, "Session error."), nil
	}
	return contract.ReauthenticateWithPassword204Response{}, nil
}

func (h *APIHandler) ReauthenticateWithGoogle(ctx context.Context, request contract.ReauthenticateWithGoogleRequestObject) (contract.ReauthenticateWithGoogleResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return reauthenticateWithGoogleError(response.CodeInternalError, "API contract error."), nil
	}
	userID, userOK := httpctx.UserID(ctx)
	sessionID, sessionOK := httpctx.SessionID(ctx)
	if !userOK || !sessionOK {
		return reauthenticateWithGoogleError(response.CodeUnauthorized, "Unauthorized."), nil
	}
	if request.Body == nil {
		return reauthenticateWithGoogleError(response.CodeInvalidRequest, "Invalid JSON body."), nil
	}
	now := time.Now().UTC()
	if err := h.Session.ValidateAuthenticationChallenge(ctx, userID, sessionID, request.Body.AuthenticationChallengeId, now); err != nil {
		if errors.Is(err, session.ErrAuthenticationChallengeInvalid) {
			return reauthenticateWithGoogleError(response.CodeInvalidAuthenticationChallenge, "Authentication challenge is invalid or expired."), nil
		}
		return reauthenticateWithGoogleError(response.CodeInternalError, "Session error."), nil
	}
	identity, header, code, message, ok := h.verifyGoogleCredential(ctx, r, request.Body.Credential, request.Body.Nonce)
	if !ok {
		return reauthenticateWithGoogleError(code, message), nil
	}
	if err := h.Auth.VerifyExternalIdentity(ctx, userID, domain.ProviderGoogle, identity.OAuthID); err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return reauthenticateWithGoogleError(response.CodeUnauthorized, "Google identity is not linked to this account."), nil
		}
		return reauthenticateWithGoogleError(response.CodeInternalError, "Authentication error."), nil
	}
	if err := h.Session.CompleteAuthenticationChallenge(ctx, userID, sessionID, request.Body.AuthenticationChallengeId, domain.AuthenticationMethodGoogle, now); err != nil {
		if errors.Is(err, session.ErrAuthenticationChallengeInvalid) {
			return reauthenticateWithGoogleError(response.CodeInvalidAuthenticationChallenge, "Authentication challenge is invalid or expired."), nil
		}
		return reauthenticateWithGoogleError(response.CodeInternalError, "Session error."), nil
	}
	return contract.ReauthenticateWithGoogle204HeadersResponse{Header: header}, nil
}

func (h *APIHandler) BeginPasskeyReauthentication(ctx context.Context, request contract.BeginPasskeyReauthenticationRequestObject) (contract.BeginPasskeyReauthenticationResponseObject, error) {
	r, requestOK := contractRequest(ctx)
	if !requestOK {
		return beginPasskeyReauthenticationError(response.CodeInternalError, "API contract error."), nil
	}
	userID, userOK := httpctx.UserID(ctx)
	sessionID, sessionOK := httpctx.SessionID(ctx)
	if !userOK || !sessionOK {
		return beginPasskeyReauthenticationError(response.CodeUnauthorized, "Unauthorized."), nil
	}
	if h.Passkeys == nil {
		return beginPasskeyReauthenticationError(response.CodeInternalError, "Passkeys are not available."), nil
	}
	if request.Body == nil {
		return beginPasskeyReauthenticationError(response.CodeInvalidRequest, "Authentication challenge required."), nil
	}
	if err := h.Session.ValidateAuthenticationChallenge(ctx, userID, sessionID, request.Body.AuthenticationChallengeId, time.Now().UTC()); err != nil {
		if errors.Is(err, session.ErrAuthenticationChallengeInvalid) {
			return beginPasskeyReauthenticationError(response.CodeInvalidAuthenticationChallenge, "Authentication challenge is invalid or expired."), nil
		}
		return beginPasskeyReauthenticationError(response.CodeInternalError, "Session error."), nil
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowPasskeyLoginAttempt(ctx, httputil.ClientIP(r))
		if err != nil || !allowed {
			return beginPasskeyReauthenticationError(response.CodeRateLimited, "Too many attempts. Please try again later."), nil
		}
	}
	optionsJSON, _, err := h.Passkeys.BeginReauthentication(ctx, userID, sessionID)
	if errors.Is(err, passkey.ErrPasskeyNotFound) {
		return beginPasskeyReauthenticationError(response.CodeNotFound, "No passkey is registered."), nil
	}
	if err != nil {
		return beginPasskeyReauthenticationError(response.CodeInternalError, "Passkey error."), nil
	}
	out, code, message, ok := passkeyOptionsResponse(optionsJSON)
	if !ok {
		return beginPasskeyReauthenticationError(code, message), nil
	}
	return contract.BeginPasskeyReauthentication200JSONResponse(out), nil
}

func (h *APIHandler) FinishPasskeyReauthentication(ctx context.Context, request contract.FinishPasskeyReauthenticationRequestObject) (contract.FinishPasskeyReauthenticationResponseObject, error) {
	r, requestOK := contractRequest(ctx)
	if !requestOK {
		return finishPasskeyReauthenticationError(response.CodeInternalError, "API contract error."), nil
	}
	userID, userOK := httpctx.UserID(ctx)
	sessionID, sessionOK := httpctx.SessionID(ctx)
	if !userOK || !sessionOK {
		return finishPasskeyReauthenticationError(response.CodeUnauthorized, "Unauthorized."), nil
	}
	if h.Passkeys == nil {
		return finishPasskeyReauthenticationError(response.CodeInternalError, "Passkeys are not available."), nil
	}
	if request.Body == nil {
		return finishPasskeyReauthenticationError(response.CodeInvalidRequest, "Invalid passkey response."), nil
	}
	now := time.Now().UTC()
	if err := h.Session.ValidateAuthenticationChallenge(ctx, userID, sessionID, request.Body.AuthenticationChallengeId, now); err != nil {
		if errors.Is(err, session.ErrAuthenticationChallengeInvalid) {
			return finishPasskeyReauthenticationError(response.CodeInvalidAuthenticationChallenge, "Authentication challenge is invalid or expired."), nil
		}
		return finishPasskeyReauthenticationError(response.CodeInternalError, "Session error."), nil
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowPasskeyLoginFinishAttempt(ctx, httputil.ClientIP(r))
		if err != nil || !allowed {
			return finishPasskeyReauthenticationError(response.CodeRateLimited, "Too many attempts. Please try again later."), nil
		}
	}
	credential, err := json.Marshal(request.Body.Credential)
	if err != nil {
		return finishPasskeyReauthenticationError(response.CodeInvalidRequest, "Invalid passkey response."), nil
	}
	if err := h.Passkeys.FinishReauthentication(ctx, userID, sessionID, request.Body.ChallengeId, credential, now); err != nil {
		if errors.Is(err, passkey.ErrPasskeyAuthenticationInvalid) {
			return finishPasskeyReauthenticationError(response.CodeInvalidRequest, "Passkey authentication failed."), nil
		}
		return finishPasskeyReauthenticationError(response.CodeInternalError, "Passkey error."), nil
	}
	if err := h.Session.CompleteAuthenticationChallenge(ctx, userID, sessionID, request.Body.AuthenticationChallengeId, domain.AuthenticationMethodPasskey, now); err != nil {
		if errors.Is(err, session.ErrAuthenticationChallengeInvalid) {
			return finishPasskeyReauthenticationError(response.CodeInvalidAuthenticationChallenge, "Authentication challenge is invalid or expired."), nil
		}
		return finishPasskeyReauthenticationError(response.CodeInternalError, "Session error."), nil
	}
	return contract.FinishPasskeyReauthentication204Response{}, nil
}

func reauthenticateWithPasswordError(code response.ErrorCode, message string) contract.ReauthenticateWithPasswordResponseObject {
	body := apiErrorBody(code, message)
	switch code {
	case response.CodeInvalidRequest:
		return contract.ReauthenticateWithPassword400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case response.CodeUnauthorized:
		return contract.ReauthenticateWithPassword401JSONResponse(body)
	case response.CodeInvalidAuthenticationChallenge:
		return contract.ReauthenticateWithPassword409JSONResponse(body)
	case response.CodeRateLimited:
		return contract.ReauthenticateWithPassword429JSONResponse(body)
	default:
		return contract.ReauthenticateWithPassword500JSONResponse(body)
	}
}

func reauthenticateWithGoogleError(code response.ErrorCode, message string) contract.ReauthenticateWithGoogleResponseObject {
	body := apiErrorBody(code, message)
	switch code {
	case response.CodeInvalidRequest:
		return contract.ReauthenticateWithGoogle400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case response.CodeUnauthorized:
		return contract.ReauthenticateWithGoogle401JSONResponse(body)
	case response.CodeNotFound:
		return contract.ReauthenticateWithGoogle404JSONResponse(body)
	case response.CodeInvalidAuthenticationChallenge:
		return contract.ReauthenticateWithGoogle409JSONResponse(body)
	default:
		return contract.ReauthenticateWithGoogle500JSONResponse(body)
	}
}

func beginPasskeyReauthenticationError(code response.ErrorCode, message string) contract.BeginPasskeyReauthenticationResponseObject {
	body := apiErrorBody(code, message)
	switch code {
	case response.CodeInvalidRequest:
		return contract.BeginPasskeyReauthentication400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case response.CodeUnauthorized:
		return contract.BeginPasskeyReauthentication401JSONResponse(body)
	case response.CodeNotFound:
		return contract.BeginPasskeyReauthentication404JSONResponse(body)
	case response.CodeRateLimited:
		return contract.BeginPasskeyReauthentication429JSONResponse(body)
	case response.CodeInvalidAuthenticationChallenge:
		return contract.BeginPasskeyReauthentication409JSONResponse(body)
	default:
		return contract.BeginPasskeyReauthentication500JSONResponse(body)
	}
}

func finishPasskeyReauthenticationError(code response.ErrorCode, message string) contract.FinishPasskeyReauthenticationResponseObject {
	body := apiErrorBody(code, message)
	switch code {
	case response.CodeInvalidRequest:
		return contract.FinishPasskeyReauthentication400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case response.CodeUnauthorized:
		return contract.FinishPasskeyReauthentication401JSONResponse(body)
	case response.CodeInvalidAuthenticationChallenge:
		return contract.FinishPasskeyReauthentication409JSONResponse(body)
	case response.CodeRateLimited:
		return contract.FinishPasskeyReauthentication429JSONResponse(body)
	default:
		return contract.FinishPasskeyReauthentication500JSONResponse(body)
	}
}
