package api

import (
	"context"
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/oauthstate"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/session"
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
	identity, header, code, message, ok := h.verifyGoogleCredential(ctx, r, request.Body.Credential, request.Body.Nonce)
	if !ok {
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
	nonce, err := oauthstate.EnsureNonce(contract.HeaderWriter(header), r)
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
		Provider: domain.ProviderGoogle,
		Email:    identity.Email,
		OAuthID:  identity.OAuthID,
	})
	if err != nil {
		code := googleLoginErrorCode(err)
		message := "Google login error."
		switch code {
		case codeAccountLinkRequired:
			message = "An account with this email already exists. Sign in with an existing method and link Google from your account."
		case response.CodeForbidden:
			message = "Google login is not allowed for this account."
		}
		return loginWithGoogleError(code, message)
	}
	accessToken, refreshToken, err := h.Session.CreateSession(ctx, user.ID, audience, r.UserAgent(), time.Now())
	switch sessionErrorCode(err) {
	case response.CodeForbidden:
		return loginWithGoogleError(response.CodeForbidden, "Account cannot access requested audience.")
	case response.CodeInternalError:
		return loginWithGoogleError(response.CodeInternalError, "Session error.")
	}
	session.SetAccessToken(contract.HeaderWriter(header), accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(contract.HeaderWriter(header), refreshToken, int(h.RefreshTTL.Seconds()))
	return contract.LoginWithGoogle200HeadersResponse{Header: header, Body: toContractAuthSession(user, accessToken, refreshToken)}
}
