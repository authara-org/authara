package api

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
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
	r, out, ok := contractRequest(ctx)
	if !ok {
		return out, nil
	}
	if _, ok := h.googleClientID(); !ok || h.Google == nil {
		return routeError(LoginWithGoogleErrors, responseCodeNotFound(), "Google login is not enabled."), nil
	}
	if request.Body == nil {
		return routeError(LoginWithGoogleErrors, responseCodeInvalidRequest(), "Invalid JSON body."), nil
	}
	credential := strings.TrimSpace(request.Body.Credential)
	nonce := strings.TrimSpace(request.Body.Nonce)
	if credential == "" || nonce == "" {
		return routeError(LoginWithGoogleErrors, responseCodeInvalidRequest(), "Google credential and nonce required."), nil
	}
	audience := token.AudienceApp
	if request.Params.Audience != nil {
		audience = token.Audience(*request.Params.Audience)
	}
	expectedNonce, ok := oauthstate.ReadNonce(r)
	if !ok || subtle.ConstantTimeCompare([]byte(nonce), []byte(expectedNonce)) != 1 {
		return routeError(LoginWithGoogleErrors, responseCodeUnauthorized(), "Invalid Google credential."), nil
	}
	identity, err := h.Google.VerifyIDToken(ctx, credential, expectedNonce)
	if err != nil {
		return routeError(LoginWithGoogleErrors, responseCodeUnauthorized(), "Invalid Google credential."), nil
	}
	header := make(http.Header)
	oauthstate.ClearNonce(contract.HeaderWriter(header))
	return h.contractGoogleLogin(ctx, r, LoginWithGoogleErrors, identity, audience, header), nil
}

func (h *APIHandler) GetGoogleLoginOptions(ctx context.Context, _ contract.GetGoogleLoginOptionsRequestObject) (contract.GetGoogleLoginOptionsResponseObject, error) {
	r, out, ok := contractRequest(ctx)
	if !ok {
		return out, nil
	}
	clientID, ok := h.googleClientID()
	if !ok {
		return routeError(GetGoogleLoginOptionsErrors, responseCodeNotFound(), "Google login is not enabled."), nil
	}
	header := make(http.Header)
	nonce, err := oauthstate.EnsureNonce(contract.HeaderWriter(header), r)
	if err != nil {
		return routeError(GetGoogleLoginOptionsErrors, responseCodeInternalError(), "Google login setup error."), nil
	}
	return contract.JSON(http.StatusOK, contract.GoogleLoginOptions{ClientId: clientID, Nonce: nonce}, header), nil
}

func (h *APIHandler) contractGoogleLogin(
	ctx context.Context,
	r *http.Request,
	routeErrors map[response.ErrorCode]response.ErrorSpec,
	identity *google.Identity,
	audience token.Audience,
	header http.Header,
) contract.Response {
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
		return routeError(routeErrors, code, message)
	}
	accessToken, refreshToken, err := h.Session.CreateSession(ctx, user.ID, audience, r.UserAgent(), time.Now())
	switch sessionErrorCode(err) {
	case response.CodeForbidden:
		return routeError(routeErrors, response.CodeForbidden, "Account cannot access requested audience.")
	case response.CodeInternalError:
		return routeError(routeErrors, response.CodeInternalError, "Session error.")
	}
	session.SetAccessToken(contract.HeaderWriter(header), accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(contract.HeaderWriter(header), refreshToken, int(h.RefreshTTL.Seconds()))
	return contract.JSON(http.StatusOK, toContractAuthSession(user, accessToken, refreshToken), header)
}
