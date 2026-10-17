package api

import (
	"context"
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/oauthstate"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/session/token"
)

func (h *APIHandler) LoginWithGoogle(ctx context.Context, request contract.LoginWithGoogleRequestObject) (contract.LoginWithGoogleResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return loginWithGoogleError(responseCodeInternalError(), "API contract error."), nil
	}
	if request.Body == nil {
		return loginWithGoogleError(responseCodeInvalidRequest(), "Invalid JSON body."), nil
	}
	audience := token.AudienceApp
	if request.Params.Audience != nil {
		audience = token.Audience(*request.Params.Audience)
	}
	identity, header, code, message, ok := h.verifyGoogleAuthorization(ctx, r, request.Body.Credential, request.Body.Nonce, request.Body.Code, request.Body.RedirectUri, request.Body.ChallengeId)
	if !ok {
		if code == response.CodeUnauthorized {
			if err := h.Auth.RecordLoginDenied(ctx, domain.AuthenticationMethodGoogle, domain.SecurityEventReasonInvalidAssertion); err != nil {
				return loginWithGoogleError(responseCodeInternalError(), "Google sign-in error."), nil
			}
		}
		return loginWithGoogleError(code, message), nil
	}
	return h.contractGoogleLogin(ctx, r, identity, audience, header), nil
}

func (h *APIHandler) GetGoogleLoginOptions(ctx context.Context, _ contract.GetGoogleLoginOptionsRequestObject) (contract.GetGoogleLoginOptionsResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return getGoogleLoginOptionsError(responseCodeInternalError(), "API contract error."), nil
	}
	clientID, ok := h.googleClientID()
	if !ok {
		return getGoogleLoginOptionsError(responseCodeNotFound(), "Google login is not enabled."), nil
	}
	header := make(http.Header)
	if tokenSessionRequested(r) {
		if h.OAuthChallenges == nil {
			return getGoogleLoginOptionsError(responseCodeInternalError(), "Google login setup error."), nil
		}
		challenge, err := h.OAuthChallenges.Create(ctx, domain.ProviderGoogle, time.Now().UTC())
		if err != nil {
			return getGoogleLoginOptionsError(responseCodeInternalError(), "Google login setup error."), nil
		}
		return contract.GetGoogleLoginOptions200HeadersResponse{
			Header: header,
			Body:   contract.GoogleLoginOptions{ClientId: clientID, Nonce: challenge.Nonce, ChallengeId: &challenge.ID},
		}, nil
	}
	nonce, err := oauthstate.Create(contract.HeaderWriter(header), r)
	if err != nil {
		return getGoogleLoginOptionsError(responseCodeInternalError(), "Google login setup error."), nil
	}
	return contract.GetGoogleLoginOptions200HeadersResponse{
		Header: header,
		Body:   contract.GoogleLoginOptions{ClientId: clientID, Nonce: nonce},
	}, nil
}

func (h *APIHandler) contractGoogleLogin(
	ctx context.Context,
	r *http.Request,
	identity *google.Identity,
	audience token.Audience,
	header http.Header,
) contract.LoginWithGoogleResponseObject {
	user, err := h.Auth.Login(ctx, auth.LoginInput{
		Provider:              domain.ProviderGoogle,
		Email:                 identity.Email,
		OAuthID:               identity.OAuthID,
		ProviderEmailVerified: identity.EmailVerified,
	})
	if err != nil {
		code := googleLoginErrorCode(err)
		message := "Google sign-in error."
		switch code {
		case codeAccountLinkRequired:
			recovery, recoveryCode, recoveryMessage, ok := h.startGoogleAccountRecoveryLink(ctx, identity)
			if !ok {
				return googleLoginErrorWithHeaders(recoveryCode, recoveryMessage, header)
			}
			message = "An account with this email already exists. Prove ownership with an existing sign-in method to connect Google."
			return googleLoginErrorWithHeaders(code, message, header, recovery)
		case response.CodeForbidden:
			message = "Google login is not allowed for this account."
		}
		return googleLoginErrorWithHeaders(code, message, header)
	}
	accessToken, refreshToken, err := h.Session.CreateSession(ctx, user.ID, audience, domain.AuthenticationMethodGoogle, r.UserAgent(), time.Now(), httputil.ClientIPString(r))
	switch sessionErrorCode(err) {
	case response.CodeForbidden:
		return googleLoginErrorWithHeaders(response.CodeForbidden, "Account cannot access requested audience.", header)
	case response.CodeInternalError:
		return googleLoginErrorWithHeaders(response.CodeInternalError, "Session error.", header)
	}
	copyHeaders(header, sessionHeader(h, r, accessToken, refreshToken))
	return contract.LoginWithGoogle200HeadersResponse{Header: header, Body: toContractAuthSession(user, accessToken, refreshToken)}
}
