package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/applestate"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/oauth/apple"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
)

func (h *APIHandler) GetAppleLoginOptions(ctx context.Context, _ contract.GetAppleLoginOptionsRequestObject) (contract.GetAppleLoginOptionsResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return getAppleLoginOptionsError(response.CodeInternalError, "API contract error."), nil
	}
	provider, ok := h.appleProvider()
	if !ok || h.Apple == nil {
		return getAppleLoginOptionsError(response.CodeNotFound, "Apple login is not enabled."), nil
	}
	header := make(http.Header)
	flow, err := applestate.Create(contract.HeaderWriter(header), r)
	if err != nil {
		return getAppleLoginOptionsError(response.CodeInternalError, "Apple login setup error."), nil
	}
	return contract.GetAppleLoginOptions200HeadersResponse{
		Header: header,
		Body: contract.AppleLoginOptions{
			ClientId: provider.ClientID, RedirectUri: provider.RedirectURI,
			State: flow.State, Nonce: flow.Nonce,
		},
	}, nil
}

func (h *APIHandler) LoginWithApple(ctx context.Context, request contract.LoginWithAppleRequestObject) (contract.LoginWithAppleResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return loginWithAppleError(response.CodeInternalError, "API contract error."), nil
	}
	if request.Body == nil {
		return loginWithAppleError(response.CodeInvalidRequest, "Invalid JSON body."), nil
	}
	audience := token.AudienceApp
	if request.Params.Audience != nil {
		audience = token.Audience(*request.Params.Audience)
	}
	result, header, code, message, ok := h.verifyAppleAuthorization(ctx, r, request.Body.Code, request.Body.State)
	if !ok {
		if code == response.CodeUnauthorized {
			if err := h.Auth.RecordLoginDenied(ctx, domain.AuthenticationMethodApple, domain.SecurityEventReasonInvalidAssertion); err != nil {
				return appleLoginErrorWithHeaders(response.CodeInternalError, "Apple sign-in error.", header), nil
			}
		}
		return appleLoginErrorWithHeaders(code, message, header), nil
	}
	if result.Identity.Email == "" || !result.Identity.EmailVerified {
		h.discardAppleAuthorization(ctx, result.RefreshToken)
		if err := h.Auth.RecordLoginDenied(ctx, domain.AuthenticationMethodApple, domain.SecurityEventReasonInvalidAssertion); err != nil {
			return appleLoginErrorWithHeaders(response.CodeInternalError, "Apple sign-in error.", header), nil
		}
		return appleLoginErrorWithHeaders(response.CodeUnauthorized, "Apple did not return a verified email address.", header), nil
	}
	invitationToken := ""
	if request.Body.InvitationToken != nil {
		invitationToken = strings.TrimSpace(*request.Body.InvitationToken)
	}
	return h.contractAppleLogin(ctx, r, result, audience, invitationToken, header), nil
}

func (h *APIHandler) contractAppleLogin(
	ctx context.Context,
	r *http.Request,
	result apple.ExchangeResult,
	audience token.Audience,
	invitationToken string,
	header http.Header,
) contract.LoginWithAppleResponseObject {
	user, err := h.Auth.LoginWithApple(ctx, auth.LoginInput{
		Provider: domain.ProviderApple, Email: result.Identity.Email,
		OAuthID: result.Identity.OAuthID, ProviderEmailVerified: result.Identity.EmailVerified,
		InvitationToken: invitationToken,
	}, result.RefreshToken)
	if err != nil {
		code := appleLoginErrorCode(err)
		message := "Apple sign-in error."
		switch code {
		case codeAccountLinkRequired:
			link, linkErr := h.Auth.StartAccountRecoveryProviderLink(ctx, auth.OAuthIdentityInput{
				Provider:              domain.ProviderApple,
				Email:                 result.Identity.Email,
				ProviderUserID:        result.Identity.OAuthID,
				ProviderEmailVerified: result.Identity.EmailVerified,
			}, time.Now().UTC())
			if linkErr != nil {
				h.discardAppleAuthorization(ctx, result.RefreshToken)
				return appleLoginErrorWithHeaders(response.CodeInternalError, "Apple sign-in error.", header)
			}
			if h.AppleCredentials == nil || h.AppleCredentials.StageProviderLink(ctx, link.ID, result.RefreshToken, link.ExpiresAt) != nil {
				h.discardAppleAuthorization(ctx, result.RefreshToken)
				return appleLoginErrorWithHeaders(response.CodeInternalError, "Apple sign-in error.", header)
			}

			u := url.URL{Path: "/auth/provider-links/confirm"}
			q := u.Query()
			q.Set("link_id", link.ID.String())
			u.RawQuery = q.Encode()
			header.Set("X-Authara-Redirect", u.String())
			message = "Confirm your existing sign-in method to connect Apple."
		case response.CodeForbidden:
			h.discardAppleAuthorization(ctx, result.RefreshToken)
			message = "Apple login is not allowed for this account."
		default:
			h.discardAppleAuthorization(ctx, result.RefreshToken)
		}
		return appleLoginErrorWithHeaders(code, message, header)
	}
	accessToken, refreshToken, err := h.Session.CreateSession(ctx, user.ID, audience, domain.AuthenticationMethodApple, r.UserAgent(), time.Now(), httputil.ClientIPString(r))
	switch sessionErrorCode(err) {
	case response.CodeForbidden:
		return appleLoginErrorWithHeaders(response.CodeForbidden, "Account cannot access requested audience.", header)
	case response.CodeInternalError:
		return appleLoginErrorWithHeaders(response.CodeInternalError, "Session error.", header)
	}
	cookiePolicy := h.sessionCookiePolicy()
	session.SetAccessToken(contract.HeaderWriter(header), accessToken, int(cookiePolicy.AccessTokenTTL.Seconds()))
	session.SetRefreshToken(contract.HeaderWriter(header), refreshToken, int(cookiePolicy.RefreshTokenTTL.Seconds()))
	return contract.LoginWithApple200HeadersResponse{Header: header, Body: toContractAuthSession(user, accessToken, refreshToken)}
}

func appleLoginErrorCode(err error) response.ErrorCode {
	switch {
	case errors.Is(err, auth.ErrAccountExistsMustLink):
		return codeAccountLinkRequired
	case errors.Is(err, auth.ErrEmailNotAllowed), errors.Is(err, auth.ErrProviderDisabled):
		return response.CodeForbidden
	default:
		return response.CodeInternalError
	}
}
