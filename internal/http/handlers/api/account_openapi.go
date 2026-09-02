package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/oauthstate"
	"github.com/authara-org/authara/internal/http/kit/validation"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/passkey"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/store"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (h *APIHandler) GetCurrentAccount(ctx context.Context, _ contract.GetCurrentAccountRequestObject) (contract.GetCurrentAccountResponseObject, error) {
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return getCurrentAccountError(responseCodeUnauthorized(), "Unauthorized."), nil
	}
	currentSessionID, ok := httpctx.SessionID(ctx)
	if !ok {
		return getCurrentAccountError(responseCodeUnauthorized(), "Unauthorized."), nil
	}

	user, err := h.Auth.GetUser(ctx, userID)
	if err != nil {
		return getCurrentAccountError(responseCodeUnauthorized(), "Unauthorized."), nil
	}
	sessions, err := h.Session.ListUserSessions(ctx, userID, currentSessionID, time.Now().UTC())
	if err != nil {
		return getCurrentAccountError(responseCodeInternalError(), "Account error."), nil
	}
	providers, err := h.Auth.ListUserAuthProviders(ctx, userID)
	if err != nil {
		return getCurrentAccountError(responseCodeInternalError(), "Account error."), nil
	}
	var passkeys []domain.Passkey
	if h.Passkeys != nil {
		passkeys, err = h.Passkeys.ListUserPasskeys(ctx, userID)
		if err != nil {
			return getCurrentAccountError(responseCodeInternalError(), "Account error."), nil
		}
	}

	out := contract.Account{
		User: contract.AuthUser{
			Id:        user.ID,
			Email:     openapi_types.Email(user.Email),
			Username:  user.Username,
			Disabled:  user.DisabledAt != nil,
			CreatedAt: user.CreatedAt,
		},
		Sessions:    make([]contract.AccountSession, 0, len(sessions)),
		AuthMethods: make([]contract.AuthMethod, 0, len(providers)),
		Passkeys:    make([]contract.AccountPasskey, 0, len(passkeys)),
	}
	for _, s := range sessions {
		out.Sessions = append(out.Sessions, contract.AccountSession{
			Id:        s.ID,
			Current:   s.ID == currentSessionID,
			CreatedAt: s.CreatedAt,
			ExpiresAt: s.ExpiresAt,
			UserAgent: s.UserAgent,
		})
	}
	for _, provider := range providers {
		out.AuthMethods = append(out.AuthMethods, contract.AuthMethod{
			Provider:  contract.AuthMethodProvider(provider.Provider),
			CreatedAt: provider.CreatedAt,
		})
	}
	for _, key := range passkeys {
		out.Passkeys = append(out.Passkeys, contract.AccountPasskey{
			Id:         key.ID,
			Name:       key.Name,
			CreatedAt:  key.CreatedAt,
			LastUsedAt: key.LastUsedAt,
		})
	}

	return contract.GetCurrentAccount200JSONResponse(out), nil
}

func (h *APIHandler) ChangeCurrentUsername(ctx context.Context, request contract.ChangeCurrentUsernameRequestObject) (contract.ChangeCurrentUsernameResponseObject, error) {
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return changeCurrentUsernameError(responseCodeUnauthorized(), "Unauthorized."), nil
	}
	if request.Body == nil {
		return changeCurrentUsernameError(responseCodeInvalidRequest(), "Invalid username."), nil
	}
	username := strings.TrimSpace(request.Body.Username)
	if err := h.Auth.ChangeUsername(ctx, userID, username); err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidUsername):
			return changeCurrentUsernameError(responseCodeInvalidRequest(), "Invalid username."), nil
		case errors.Is(err, auth.ErrUsernameTaken):
			return changeCurrentUsernameError(codeUsernameTaken, "Username is already taken."), nil
		default:
			return changeCurrentUsernameError(responseCodeInternalError(), "Account error."), nil
		}
	}
	return contract.ChangeCurrentUsername204Response{}, nil
}

func (h *APIHandler) StartCurrentUserEmailChange(ctx context.Context, request contract.StartCurrentUserEmailChangeRequestObject) (contract.StartCurrentUserEmailChangeResponseObject, error) {
	if !h.challengeAvailable() {
		return startCurrentUserEmailChangeError(responseCodeNotFound(), "Challenge verification is not enabled."), nil
	}
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return startCurrentUserEmailChangeError(responseCodeUnauthorized(), "Unauthorized."), nil
	}
	if request.Body == nil {
		return startCurrentUserEmailChangeError(responseCodeInvalidRequest(), "Invalid email address."), nil
	}
	newEmail := strings.ToLower(strings.TrimSpace(string(request.Body.NewEmail)))
	if !validation.IsValidEmail(newEmail) {
		return startCurrentUserEmailChangeError(responseCodeInvalidRequest(), "Invalid email address."), nil
	}
	user, err := h.Auth.GetUser(ctx, userID)
	if err != nil {
		return startCurrentUserEmailChangeError(responseCodeUnauthorized(), "Unauthorized."), nil
	}
	if strings.EqualFold(user.Email, newEmail) {
		return startCurrentUserEmailChangeError(responseCodeInvalidRequest(), "New email address must be different."), nil
	}

	now := time.Now().UTC()
	var challengeID openapi_types.UUID
	exists, err := h.Auth.UserExistsByEmail(ctx, newEmail)
	if err == nil && exists {
		challengeID, err = h.Challenge.CreateOpaqueChallenge(ctx, now, domain.ChallengePurposeEmailChange, newEmail)
	} else if err == nil {
		challengeID, err = h.Challenge.CreateEmailChangeChallenge(ctx, challenge.CreateEmailChangeChallengeInput{
			UserID:   user.ID,
			OldEmail: user.Email,
			NewEmail: newEmail,
		}, now)
	}
	if err != nil {
		return startCurrentUserEmailChangeError(responseCodeInternalError(), "Email change error."), nil
	}
	return contract.StartCurrentUserEmailChange202JSONResponse{ChallengeId: challengeID}, nil
}

func (h *APIHandler) VerifyCurrentUserEmailChange(ctx context.Context, request contract.VerifyCurrentUserEmailChangeRequestObject) (contract.VerifyCurrentUserEmailChangeResponseObject, error) {
	if !h.challengeAvailable() {
		return verifyCurrentUserEmailChangeError(responseCodeNotFound(), "Challenge verification is not enabled."), nil
	}
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return verifyCurrentUserEmailChangeError(responseCodeUnauthorized(), "Unauthorized."), nil
	}
	if request.Body == nil || !isSixDigitCode(strings.TrimSpace(request.Body.Code)) {
		return verifyCurrentUserEmailChangeError(responseCodeInvalidRequest(), "Invalid challenge request."), nil
	}
	if r, ok := contractRequest(ctx); ok && h.Limiter != nil {
		allowed, err := h.Limiter.AllowChallengeVerifyAttempt(ctx, httputil.ClientIP(r))
		if err != nil || !allowed {
			return verifyCurrentUserEmailChangeError(responseCodeRateLimited(), "Too many verification attempts. Please try again later."), nil
		}
	}

	result, err := h.Challenge.VerifyEmailChangeChallenge(ctx, request.Body.ChallengeId, strings.TrimSpace(request.Body.Code), h.Verification, time.Now().UTC())
	if err != nil {
		if isExpectedEmailChangeVerifyError(err) {
			return verifyCurrentUserEmailChangeError(responseCodeInvalidRequest(), "Invalid or expired verification code."), nil
		}
		return verifyCurrentUserEmailChangeError(responseCodeInternalError(), "Challenge error."), nil
	}
	if result.Action.UserID != userID {
		return verifyCurrentUserEmailChangeError(responseCodeForbidden(), "Email change does not belong to the current user."), nil
	}
	if err := h.Challenge.ExecuteEmailChange(ctx, result.Action, time.Now().UTC()); err != nil {
		return verifyCurrentUserEmailChangeError(responseCodeInternalError(), "Email change error."), nil
	}
	return contract.VerifyCurrentUserEmailChange204Response{}, nil
}

func (h *APIHandler) ChangeCurrentUserPassword(ctx context.Context, request contract.ChangeCurrentUserPasswordRequestObject) (contract.ChangeCurrentUserPasswordResponseObject, error) {
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return changeCurrentUserPasswordError(responseCodeUnauthorized(), "Unauthorized."), nil
	}
	if request.Body == nil || !validation.IsValidPassword(request.Body.NewPassword) {
		return changeCurrentUserPasswordError(responseCodeInvalidRequest(), "Invalid password."), nil
	}
	passwordHash, err := auth.Hash(request.Body.NewPassword)
	if err != nil {
		return changeCurrentUserPasswordError(responseCodeInternalError(), "Password error."), nil
	}
	if err := h.Auth.ChangePassword(ctx, userID, request.Body.CurrentPassword, passwordHash); err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			return changeCurrentUserPasswordError(responseCodeInvalidRequest(), "Current password is incorrect."), nil
		case errors.Is(err, store.ErrorAuthProviderNotFound):
			return changeCurrentUserPasswordError(responseCodeNotFound(), "No password is set for this account."), nil
		default:
			return changeCurrentUserPasswordError(responseCodeInternalError(), "Password error."), nil
		}
	}
	return contract.ChangeCurrentUserPassword204Response{}, nil
}

func (h *APIHandler) AddCurrentUserPassword(ctx context.Context, request contract.AddCurrentUserPasswordRequestObject) (contract.AddCurrentUserPasswordResponseObject, error) {
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return addCurrentUserPasswordError(responseCodeUnauthorized(), "Unauthorized."), nil
	}
	if request.Body == nil || !validation.IsValidPassword(request.Body.Password) {
		return addCurrentUserPasswordError(responseCodeInvalidRequest(), "Invalid password."), nil
	}
	passwordHash, err := auth.Hash(request.Body.Password)
	if err != nil {
		return addCurrentUserPasswordError(responseCodeInternalError(), "Password error."), nil
	}
	if err := h.Auth.AddPassword(ctx, userID, passwordHash); err != nil {
		if errors.Is(err, auth.ErrPasswordAlreadyExists) {
			return addCurrentUserPasswordError(codePasswordAlreadyExists, "A password is already set for this account."), nil
		}
		return addCurrentUserPasswordError(responseCodeInternalError(), "Password error."), nil
	}
	return contract.AddCurrentUserPassword204Response{}, nil
}

func (h *APIHandler) LinkCurrentUserGoogle(ctx context.Context, request contract.LinkCurrentUserGoogleRequestObject) (contract.LinkCurrentUserGoogleResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return linkCurrentUserGoogleError(responseCodeInternalError(), "API contract error."), nil
	}
	userID, userOK := httpctx.UserID(ctx)
	sessionID, sessionOK := httpctx.SessionID(ctx)
	if !userOK || !sessionOK {
		return linkCurrentUserGoogleError(responseCodeUnauthorized(), "Unauthorized."), nil
	}
	if _, ok := h.googleClientID(); !ok || h.Google == nil {
		return linkCurrentUserGoogleError(responseCodeNotFound(), "Google login is not enabled."), nil
	}
	if request.Body == nil {
		return linkCurrentUserGoogleError(responseCodeInvalidRequest(), "Invalid Google credential."), nil
	}
	nonce := strings.TrimSpace(request.Body.Nonce)
	expectedNonce, ok := oauthstate.ReadNonce(r)
	if !ok || nonce == "" || subtle.ConstantTimeCompare([]byte(nonce), []byte(expectedNonce)) != 1 {
		return linkCurrentUserGoogleError(responseCodeUnauthorized(), "Invalid Google credential."), nil
	}
	identity, err := h.Google.VerifyIDToken(ctx, strings.TrimSpace(request.Body.Credential), expectedNonce)
	if err != nil {
		return linkCurrentUserGoogleError(responseCodeUnauthorized(), "Invalid Google credential."), nil
	}
	linkID, err := h.Auth.StartProviderLink(ctx, userID, sessionID, domain.ProviderGoogle, time.Now().UTC())
	if err == nil {
		err = h.Auth.CompleteProviderLink(ctx, linkID, userID, sessionID, domain.ProviderGoogle, identity.OAuthID, identity.Email, identity.EmailVerified, time.Now().UTC())
	}
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrAuthProviderAlreadyLinked), errors.Is(err, auth.ErrAuthProviderAlreadyLinkedToUser):
			return linkCurrentUserGoogleError(codeAuthMethodAlreadyLinked, "Google is already linked."), nil
		case errors.Is(err, auth.ErrProviderEmailNotVerified), errors.Is(err, auth.ErrProviderDisabled):
			return linkCurrentUserGoogleError(responseCodeForbidden(), "Google account cannot be linked."), nil
		default:
			return linkCurrentUserGoogleError(responseCodeInternalError(), "Could not link Google."), nil
		}
	}
	header := make(http.Header)
	oauthstate.ClearNonce(contract.HeaderWriter(header))
	return contract.LinkCurrentUserGoogle204HeadersResponse{Header: header}, nil
}

func (h *APIHandler) UnlinkCurrentUserAuthMethod(ctx context.Context, request contract.UnlinkCurrentUserAuthMethodRequestObject) (contract.UnlinkCurrentUserAuthMethodResponseObject, error) {
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return unlinkCurrentUserAuthMethodError(responseCodeUnauthorized(), "Unauthorized."), nil
	}
	provider := domain.Provider(request.Provider)
	if provider != domain.ProviderPassword && provider != domain.ProviderGoogle {
		return unlinkCurrentUserAuthMethodError(responseCodeInvalidRequest(), "Invalid authentication method."), nil
	}
	if err := h.Auth.UnlinkAuthProvider(ctx, userID, provider); err != nil {
		switch {
		case errors.Is(err, auth.ErrCannotRemoveLastAuthMethod):
			return unlinkCurrentUserAuthMethodError(codeCannotRemoveLastAuthMethod, "You need at least one sign-in method."), nil
		case errors.Is(err, store.ErrorAuthProviderNotFound):
			return unlinkCurrentUserAuthMethodError(responseCodeNotFound(), "Authentication method not found."), nil
		default:
			return unlinkCurrentUserAuthMethodError(responseCodeInternalError(), "Could not unlink authentication method."), nil
		}
	}
	return contract.UnlinkCurrentUserAuthMethod204Response{}, nil
}

func (h *APIHandler) DeleteCurrentUserPasskey(ctx context.Context, request contract.DeleteCurrentUserPasskeyRequestObject) (contract.DeleteCurrentUserPasskeyResponseObject, error) {
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return deleteCurrentUserPasskeyError(responseCodeUnauthorized(), "Unauthorized."), nil
	}
	if h.Passkeys == nil {
		return deleteCurrentUserPasskeyError(responseCodeInternalError(), "Passkeys are not available."), nil
	}
	if err := h.Passkeys.DeletePasskey(ctx, userID, request.PasskeyID); err != nil {
		switch {
		case errors.Is(err, passkey.ErrCannotRemoveLastAuthMethod):
			return deleteCurrentUserPasskeyError(codeCannotRemoveLastAuthMethod, "You need at least one sign-in method."), nil
		case errors.Is(err, passkey.ErrPasskeyNotFound):
			return deleteCurrentUserPasskeyError(responseCodeNotFound(), "Passkey not found."), nil
		default:
			return deleteCurrentUserPasskeyError(responseCodeInternalError(), "Could not delete passkey."), nil
		}
	}
	return contract.DeleteCurrentUserPasskey204Response{}, nil
}

func (h *APIHandler) RevokeCurrentUserSession(ctx context.Context, request contract.RevokeCurrentUserSessionRequestObject) (contract.RevokeCurrentUserSessionResponseObject, error) {
	userID, ok := httpctx.UserID(ctx)
	if !ok {
		return revokeCurrentUserSessionError(responseCodeUnauthorized(), "Unauthorized."), nil
	}
	if err := h.Session.RevokeUserSession(ctx, userID, request.SessionID, time.Now().UTC()); err != nil {
		switch {
		case errors.Is(err, session.ErrForbidden):
			return revokeCurrentUserSessionError(responseCodeForbidden(), "You are not allowed to revoke this session."), nil
		case errors.Is(err, store.ErrSessionNotFound):
			return revokeCurrentUserSessionError(responseCodeNotFound(), "Session not found."), nil
		default:
			return revokeCurrentUserSessionError(responseCodeInternalError(), "Could not revoke session."), nil
		}
	}
	return contract.RevokeCurrentUserSession204Response{}, nil
}

func (h *APIHandler) RevokeCurrentUserOtherSessions(ctx context.Context, _ contract.RevokeCurrentUserOtherSessionsRequestObject) (contract.RevokeCurrentUserOtherSessionsResponseObject, error) {
	userID, userOK := httpctx.UserID(ctx)
	currentSessionID, sessionOK := httpctx.SessionID(ctx)
	if !userOK || !sessionOK {
		return revokeCurrentUserOtherSessionsError(responseCodeUnauthorized(), "Unauthorized."), nil
	}
	if err := h.Session.RevokeOtherUserSessions(ctx, userID, currentSessionID, time.Now().UTC()); err != nil {
		if errors.Is(err, session.ErrForbidden) {
			return revokeCurrentUserOtherSessionsError(responseCodeForbidden(), "You are not allowed to revoke these sessions."), nil
		}
		return revokeCurrentUserOtherSessionsError(responseCodeInternalError(), "Could not revoke sessions."), nil
	}
	return contract.RevokeCurrentUserOtherSessions204Response{}, nil
}
