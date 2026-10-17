package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/oauthstate"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/oauth/microsoft"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/google/uuid"
)

func (h *APIHandler) microsoftClientID() (string, bool) {
	for _, provider := range h.OAuthProviders.Providers {
		if provider.Name == domain.ProviderMicrosoft && provider.ClientID != "" {
			return provider.ClientID, true
		}
	}
	return "", false
}

func (h *APIHandler) verifyMicrosoftAuthorization(
	ctx context.Context,
	r *http.Request,
	credential string,
	nonce string,
	challengeIDs ...*uuid.UUID,
) (*microsoft.Identity, http.Header, response.ErrorCode, string, bool, bool) {
	var challengeID *uuid.UUID
	if len(challengeIDs) > 0 {
		challengeID = challengeIDs[0]
	}
	if _, ok := h.microsoftClientID(); !ok || h.Microsoft == nil {
		return nil, nil, response.CodeNotFound, "Microsoft login is not enabled.", false, false
	}
	credential = strings.TrimSpace(credential)
	nonce = strings.TrimSpace(nonce)
	if credential == "" || nonce == "" {
		return nil, nil, response.CodeInvalidRequest, "Microsoft credential and nonce required.", false, false
	}
	header := make(http.Header)
	expectedNonce := ""
	if challengeID != nil && h.OAuthChallenges != nil {
		challenge, err := h.OAuthChallenges.Consume(ctx, *challengeID, domain.ProviderMicrosoft, "", time.Now().UTC())
		if err != nil && !errors.Is(err, oauth.ErrInvalidAuthenticationChallenge) {
			return nil, header, response.CodeInternalError, "Microsoft login setup error.", false, false
		}
		if err == nil {
			expectedNonce = challenge.Nonce
		}
	} else if challengeID == nil {
		expectedNonce, _ = oauthstate.Consume(contract.HeaderWriter(header), r, nonce)
	}
	if expectedNonce == "" || subtle.ConstantTimeCompare([]byte(nonce), []byte(expectedNonce)) != 1 {
		return nil, header, response.CodeUnauthorized, "Invalid Microsoft credential.", false, false
	}
	if h.Limiter != nil {
		ip := httputil.ClientIP(r)
		allowed, err := h.Limiter.AllowOAuthAttempt(ctx, ip)
		if code, message, ok := h.rateLimitResult(allowed, err, "Too many Microsoft authentication attempts. Please try again later."); !ok {
			if code == response.CodeRateLimited {
				code = response.CodeUnauthorized
				message = "Invalid Microsoft credential."
			}
			return nil, header, code, message, false, code == response.CodeUnauthorized
		}
	}
	identity, err := h.Microsoft.VerifyIDToken(ctx, credential, expectedNonce)
	if err != nil {
		logger := h.Logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.WarnContext(ctx, "microsoft credential rejected", "err", err)
		return nil, header, response.CodeUnauthorized, "Invalid Microsoft credential.", false, false
	}
	return identity, header, "", "", true, false
}

func (h *APIHandler) GetMicrosoftLoginOptions(ctx context.Context, _ contract.GetMicrosoftLoginOptionsRequestObject) (contract.GetMicrosoftLoginOptionsResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return getMicrosoftLoginOptionsError(response.CodeInternalError, "API contract error."), nil
	}
	clientID, ok := h.microsoftClientID()
	if !ok || h.Microsoft == nil {
		return getMicrosoftLoginOptionsError(response.CodeNotFound, "Microsoft login is not enabled."), nil
	}
	header := make(http.Header)
	if tokenSessionRequested(r) {
		if h.OAuthChallenges == nil {
			return getMicrosoftLoginOptionsError(response.CodeInternalError, "Microsoft login setup error."), nil
		}
		challenge, err := h.OAuthChallenges.Create(ctx, domain.ProviderMicrosoft, time.Now().UTC())
		if err != nil {
			return getMicrosoftLoginOptionsError(response.CodeInternalError, "Microsoft login setup error."), nil
		}
		return contract.GetMicrosoftLoginOptions200HeadersResponse{
			Header: header,
			Body: contract.MicrosoftLoginOptions{
				ClientId: clientID, Authority: h.Microsoft.Authority(), Nonce: challenge.Nonce, ChallengeId: &challenge.ID,
			},
		}, nil
	}
	nonce, err := oauthstate.Create(contract.HeaderWriter(header), r)
	if err != nil {
		return getMicrosoftLoginOptionsError(response.CodeInternalError, "Microsoft login setup error."), nil
	}
	return contract.GetMicrosoftLoginOptions200HeadersResponse{
		Header: header,
		Body: contract.MicrosoftLoginOptions{
			ClientId:  clientID,
			Authority: h.Microsoft.Authority(),
			Nonce:     nonce,
		},
	}, nil
}

func (h *APIHandler) LoginWithMicrosoft(ctx context.Context, request contract.LoginWithMicrosoftRequestObject) (contract.LoginWithMicrosoftResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return loginWithMicrosoftError(response.CodeInternalError, "API contract error."), nil
	}
	if request.Body == nil {
		return loginWithMicrosoftError(response.CodeInvalidRequest, "Invalid JSON body."), nil
	}
	audience := token.AudienceApp
	if request.Params.Audience != nil {
		audience = token.Audience(*request.Params.Audience)
	}
	identity, header, code, message, ok, throttled := h.verifyMicrosoftAuthorization(ctx, r, request.Body.Credential, request.Body.Nonce, request.Body.ChallengeId)
	if !ok {
		if code == response.CodeUnauthorized && !throttled {
			if err := h.Auth.RecordLoginDenied(ctx, domain.AuthenticationMethodMicrosoft, domain.SecurityEventReasonInvalidAssertion); err != nil {
				return microsoftLoginErrorWithHeaders(response.CodeInternalError, "Microsoft sign-in error.", header), nil
			}
		}
		return microsoftLoginErrorWithHeaders(code, message, header), nil
	}
	invitationToken := ""
	if request.Body.InvitationToken != nil {
		invitationToken = strings.TrimSpace(*request.Body.InvitationToken)
	}
	user, err := h.Auth.Login(ctx, auth.LoginInput{
		Provider: domain.ProviderMicrosoft, Email: identity.Email, OAuthID: identity.OAuthID,
		ProviderEmailVerified: identity.EmailVerified, InvitationToken: invitationToken,
	})
	if err != nil {
		code := microsoftLoginErrorCode(err)
		message := "Microsoft sign-in error."
		if code == codeAccountLinkRequired {
			recovery, recoveryCode, recoveryMessage, recoveryOK := h.startAccountRecoveryLink(ctx, auth.OAuthIdentityInput{
				Provider: domain.ProviderMicrosoft, Email: identity.Email,
				ProviderUserID: identity.OAuthID, ProviderEmailVerified: identity.EmailVerified,
			})
			if !recoveryOK {
				return microsoftLoginErrorWithHeaders(recoveryCode, recoveryMessage, header), nil
			}
			return microsoftLoginErrorWithHeaders(code, "An account with this email already exists. Prove ownership with an existing sign-in method to connect Microsoft.", header, recovery), nil
		}
		if code == response.CodeForbidden {
			message = "Microsoft login is not allowed for this account."
		}
		return microsoftLoginErrorWithHeaders(code, message, header), nil
	}
	accessToken, refreshToken, err := h.Session.CreateSession(ctx, user.ID, audience, domain.AuthenticationMethodMicrosoft, r.UserAgent(), time.Now(), httputil.ClientIPString(r))
	switch sessionErrorCode(err) {
	case response.CodeForbidden:
		return microsoftLoginErrorWithHeaders(response.CodeForbidden, "Account cannot access requested audience.", header), nil
	case response.CodeInternalError:
		return microsoftLoginErrorWithHeaders(response.CodeInternalError, "Session error.", header), nil
	}
	copyHeaders(header, sessionHeader(h, r, accessToken, refreshToken))
	body := toContractAuthSession(user, accessToken, refreshToken)
	if !tokenSessionRequested(r) {
		body = toContractCookieAuthSession(user)
	}
	return contract.LoginWithMicrosoft200HeadersResponse{Header: header, Body: body}, nil
}

func (h *APIHandler) LinkCurrentUserMicrosoft(ctx context.Context, request contract.LinkCurrentUserMicrosoftRequestObject) (contract.LinkCurrentUserMicrosoftResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return linkCurrentUserMicrosoftError(response.CodeInternalError, "API contract error."), nil
	}
	userID, userOK := httpctx.UserID(ctx)
	sessionID, sessionOK := httpctx.SessionID(ctx)
	if !userOK || !sessionOK {
		return linkCurrentUserMicrosoftError(response.CodeUnauthorized, "Unauthorized."), nil
	}
	if request.Body == nil {
		return linkCurrentUserMicrosoftError(response.CodeInvalidRequest, "Invalid Microsoft credential."), nil
	}
	identity, header, code, message, ok, _ := h.verifyMicrosoftAuthorization(ctx, r, request.Body.Credential, request.Body.Nonce, request.Body.ChallengeId)
	if !ok {
		return microsoftLinkErrorWithHeaders(code, message, header), nil
	}
	linkID, err := h.Auth.StartProviderLink(ctx, userID, sessionID, domain.ProviderMicrosoft, time.Now().UTC())
	if err == nil {
		err = h.Auth.CompleteProviderLink(ctx, linkID, userID, sessionID, domain.ProviderMicrosoft, identity.OAuthID, identity.Email, identity.EmailVerified, time.Now().UTC())
	}
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrAuthProviderAlreadyLinked), errors.Is(err, auth.ErrAuthProviderAlreadyLinkedToUser):
			return microsoftLinkErrorWithHeaders(codeAuthMethodAlreadyLinked, "Microsoft is already linked.", header), nil
		case errors.Is(err, auth.ErrProviderDisabled):
			return microsoftLinkErrorWithHeaders(response.CodeForbidden, "Microsoft account cannot be linked.", header), nil
		default:
			return microsoftLinkErrorWithHeaders(response.CodeInternalError, "Could not link Microsoft.", header), nil
		}
	}
	return contract.LinkCurrentUserMicrosoft204HeadersResponse{Header: header}, nil
}

func (h *APIHandler) ReauthenticateWithMicrosoft(ctx context.Context, request contract.ReauthenticateWithMicrosoftRequestObject) (contract.ReauthenticateWithMicrosoftResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return reauthenticateWithMicrosoftError(response.CodeInternalError, "API contract error."), nil
	}
	userID, userOK := httpctx.UserID(ctx)
	sessionID, sessionOK := httpctx.SessionID(ctx)
	if !userOK || !sessionOK {
		return reauthenticateWithMicrosoftError(response.CodeUnauthorized, "Unauthorized."), nil
	}
	if request.Body == nil {
		return reauthenticateWithMicrosoftError(response.CodeInvalidRequest, "Invalid JSON body."), nil
	}
	now := time.Now().UTC()
	if err := h.Session.ValidateAuthenticationChallenge(ctx, userID, sessionID, request.Body.AuthenticationChallengeId, now); err != nil {
		if errors.Is(err, session.ErrAuthenticationChallengeInvalid) {
			return reauthenticateWithMicrosoftError(response.CodeInvalidAuthenticationChallenge, "Authentication challenge is invalid or expired."), nil
		}
		return reauthenticateWithMicrosoftError(response.CodeInternalError, "Session error."), nil
	}
	identity, header, code, message, ok, throttled := h.verifyMicrosoftAuthorization(ctx, r, request.Body.Credential, request.Body.Nonce, request.Body.ChallengeId)
	if !ok {
		if code == response.CodeUnauthorized && !throttled {
			if err := h.Session.RecordReauthenticationDenied(ctx, userID, sessionID, domain.AuthenticationMethodMicrosoft, domain.SecurityEventReasonInvalidAssertion); err != nil {
				return microsoftReauthenticationErrorWithHeaders(response.CodeInternalError, "Authentication error.", header), nil
			}
		}
		return microsoftReauthenticationErrorWithHeaders(code, message, header), nil
	}
	if err := h.Auth.VerifyExternalIdentity(ctx, userID, domain.ProviderMicrosoft, identity.OAuthID); err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			if recordErr := h.Session.RecordReauthenticationDenied(ctx, userID, sessionID, domain.AuthenticationMethodMicrosoft, domain.SecurityEventReasonInvalidCredentials); recordErr != nil {
				return microsoftReauthenticationErrorWithHeaders(response.CodeInternalError, "Authentication error.", header), nil
			}
			return microsoftReauthenticationErrorWithHeaders(response.CodeUnauthorized, "Microsoft identity is not linked to this account.", header), nil
		}
		return microsoftReauthenticationErrorWithHeaders(response.CodeInternalError, "Authentication error.", header), nil
	}
	if err := h.Session.CompleteAuthenticationChallenge(ctx, userID, sessionID, request.Body.AuthenticationChallengeId, domain.AuthenticationMethodMicrosoft, now); err != nil {
		if errors.Is(err, session.ErrAuthenticationChallengeInvalid) {
			return microsoftReauthenticationErrorWithHeaders(response.CodeInvalidAuthenticationChallenge, "Authentication challenge is invalid or expired.", header), nil
		}
		return microsoftReauthenticationErrorWithHeaders(response.CodeInternalError, "Session error.", header), nil
	}
	return contract.ReauthenticateWithMicrosoft204HeadersResponse{Header: header}, nil
}

func (h *APIHandler) CompleteAccountRecoveryLinkWithMicrosoft(ctx context.Context, request contract.CompleteAccountRecoveryLinkWithMicrosoftRequestObject) (contract.CompleteAccountRecoveryLinkWithMicrosoftResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return completeAccountRecoveryLinkWithMicrosoftError(response.CodeInternalError, "API contract error."), nil
	}
	if request.Body == nil {
		return completeAccountRecoveryLinkWithMicrosoftError(response.CodeInvalidRequest, "Invalid JSON body."), nil
	}
	_, targetUser, code, message, ok := h.recoveryLinkAndUser(ctx, request.LinkID)
	if !ok {
		return completeAccountRecoveryLinkWithMicrosoftError(code, message), nil
	}
	invitationToken := optionalString(request.Body.InvitationToken)
	if invitationToken != "" {
		if code, message, ok := h.validateRecoveryInvitation(ctx, invitationToken, targetUser.Email); !ok {
			return completeAccountRecoveryLinkWithMicrosoftError(code, message), nil
		}
	}
	identity, nonceHeader, code, message, ok, _ := h.verifyMicrosoftAuthorization(ctx, r, request.Body.Credential, request.Body.Nonce, request.Body.ChallengeId)
	if !ok {
		return microsoftRecoveryErrorWithHeaders(code, message, nonceHeader), nil
	}
	user, err := h.Auth.CompleteAccountRecoveryProviderLinkWithProviderProof(ctx, request.LinkID, domain.ProviderMicrosoft, identity.OAuthID, time.Now().UTC())
	if err != nil {
		code, message := recoveryError(err)
		return microsoftRecoveryErrorWithHeaders(code, message, nonceHeader), nil
	}
	audience := token.AudienceApp
	if request.Params.Audience != nil {
		audience = token.Audience(*request.Params.Audience)
	}
	body, header, code, message, ok := h.finishRecoverySession(ctx, r, user, invitationToken, audience, domain.AuthenticationMethodMicrosoft)
	if !ok {
		return microsoftRecoveryErrorWithHeaders(code, message, nonceHeader), nil
	}
	if !tokenSessionRequested(r) {
		body = toContractCookieAuthSession(user)
	}
	copyHeaders(header, nonceHeader)
	return contract.CompleteAccountRecoveryLinkWithMicrosoft200HeadersResponse{Header: header, Body: body}, nil
}

func microsoftLoginErrorCode(err error) response.ErrorCode {
	switch {
	case errors.Is(err, auth.ErrAccountExistsMustLink):
		return codeAccountLinkRequired
	case errors.Is(err, auth.ErrEmailNotAllowed), errors.Is(err, auth.ErrProviderDisabled):
		return response.CodeForbidden
	default:
		return response.CodeInternalError
	}
}

type microsoftLoginHeadersResponse struct {
	header   http.Header
	response contract.LoginWithMicrosoftResponseObject
}

func (r microsoftLoginHeadersResponse) VisitLoginWithMicrosoftResponse(w http.ResponseWriter) error {
	copyResponseHeaders(w.Header(), r.header)
	return r.response.VisitLoginWithMicrosoftResponse(w)
}

type microsoftLinkHeadersResponse struct {
	header   http.Header
	response contract.LinkCurrentUserMicrosoftResponseObject
}

func (r microsoftLinkHeadersResponse) VisitLinkCurrentUserMicrosoftResponse(w http.ResponseWriter) error {
	copyResponseHeaders(w.Header(), r.header)
	return r.response.VisitLinkCurrentUserMicrosoftResponse(w)
}

type microsoftReauthenticationHeadersResponse struct {
	header   http.Header
	response contract.ReauthenticateWithMicrosoftResponseObject
}

func (r microsoftReauthenticationHeadersResponse) VisitReauthenticateWithMicrosoftResponse(w http.ResponseWriter) error {
	copyResponseHeaders(w.Header(), r.header)
	return r.response.VisitReauthenticateWithMicrosoftResponse(w)
}

type microsoftRecoveryHeadersResponse struct {
	header   http.Header
	response contract.CompleteAccountRecoveryLinkWithMicrosoftResponseObject
}

func (r microsoftRecoveryHeadersResponse) VisitCompleteAccountRecoveryLinkWithMicrosoftResponse(w http.ResponseWriter) error {
	copyResponseHeaders(w.Header(), r.header)
	return r.response.VisitCompleteAccountRecoveryLinkWithMicrosoftResponse(w)
}

func microsoftLoginErrorWithHeaders(code response.ErrorCode, message string, header http.Header, recovery ...contract.AccountRecoveryLink) contract.LoginWithMicrosoftResponseObject {
	return microsoftLoginHeadersResponse{header: header, response: loginWithMicrosoftError(code, message, recovery...)}
}

func microsoftLinkErrorWithHeaders(code response.ErrorCode, message string, header http.Header) contract.LinkCurrentUserMicrosoftResponseObject {
	return microsoftLinkHeadersResponse{header: header, response: linkCurrentUserMicrosoftError(code, message)}
}

func microsoftReauthenticationErrorWithHeaders(code response.ErrorCode, message string, header http.Header) contract.ReauthenticateWithMicrosoftResponseObject {
	return microsoftReauthenticationHeadersResponse{header: header, response: reauthenticateWithMicrosoftError(code, message)}
}

func microsoftRecoveryErrorWithHeaders(code response.ErrorCode, message string, header http.Header) contract.CompleteAccountRecoveryLinkWithMicrosoftResponseObject {
	return microsoftRecoveryHeadersResponse{header: header, response: completeAccountRecoveryLinkWithMicrosoftError(code, message)}
}

func getMicrosoftLoginOptionsError(code response.ErrorCode, message string) contract.GetMicrosoftLoginOptionsResponseObject {
	spec := mustRouteError(GetMicrosoftLoginOptionsErrors, code)
	body := apiErrorBody(spec.Code, message)
	if spec.Status == http.StatusNotFound {
		return contract.GetMicrosoftLoginOptions404JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	}
	return contract.GetMicrosoftLoginOptions500JSONResponse(body)
}

func loginWithMicrosoftError(code response.ErrorCode, message string, recovery ...contract.AccountRecoveryLink) contract.LoginWithMicrosoftResponseObject {
	spec := mustRouteError(LoginWithMicrosoftErrors, code)
	body := apiErrorBody(spec.Code, message)
	if len(recovery) != 0 {
		body.Recovery = &recovery[0]
	}
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.LoginWithMicrosoft400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.LoginWithMicrosoft401JSONResponse(body)
	case http.StatusForbidden:
		return contract.LoginWithMicrosoft403JSONResponse(body)
	case http.StatusNotFound:
		return contract.LoginWithMicrosoft404JSONResponse(body)
	case http.StatusConflict:
		return contract.LoginWithMicrosoft409JSONResponse(body)
	default:
		return contract.LoginWithMicrosoft500JSONResponse(body)
	}
}

func linkCurrentUserMicrosoftError(code response.ErrorCode, message string) contract.LinkCurrentUserMicrosoftResponseObject {
	spec := mustRouteError(LinkCurrentUserMicrosoftErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.LinkCurrentUserMicrosoft400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.LinkCurrentUserMicrosoft401JSONResponse(body)
	case http.StatusForbidden:
		return contract.LinkCurrentUserMicrosoft403JSONResponse(body)
	case http.StatusNotFound:
		return contract.LinkCurrentUserMicrosoft404JSONResponse(body)
	case http.StatusConflict:
		return contract.LinkCurrentUserMicrosoft409JSONResponse(body)
	default:
		return contract.LinkCurrentUserMicrosoft500JSONResponse(body)
	}
}

func reauthenticateWithMicrosoftError(code response.ErrorCode, message string) contract.ReauthenticateWithMicrosoftResponseObject {
	body := apiErrorBody(code, message)
	switch code {
	case response.CodeInvalidRequest:
		return contract.ReauthenticateWithMicrosoft400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case response.CodeUnauthorized:
		return contract.ReauthenticateWithMicrosoft401JSONResponse(body)
	case response.CodeNotFound:
		return contract.ReauthenticateWithMicrosoft404JSONResponse(body)
	case response.CodeInvalidAuthenticationChallenge:
		return contract.ReauthenticateWithMicrosoft409JSONResponse(body)
	default:
		return contract.ReauthenticateWithMicrosoft500JSONResponse(body)
	}
}

func completeAccountRecoveryLinkWithMicrosoftError(code response.ErrorCode, message string) contract.CompleteAccountRecoveryLinkWithMicrosoftResponseObject {
	spec := mustRouteError(CompleteAccountRecoveryLinkWithMicrosoftErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.CompleteAccountRecoveryLinkWithMicrosoft400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.CompleteAccountRecoveryLinkWithMicrosoft401JSONResponse(body)
	case http.StatusForbidden:
		return contract.CompleteAccountRecoveryLinkWithMicrosoft403JSONResponse(body)
	case http.StatusNotFound:
		return contract.CompleteAccountRecoveryLinkWithMicrosoft404JSONResponse(body)
	case http.StatusConflict:
		return contract.CompleteAccountRecoveryLinkWithMicrosoft409JSONResponse(body)
	default:
		return contract.CompleteAccountRecoveryLinkWithMicrosoft500JSONResponse(body)
	}
}
