package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/applestate"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/oauth/apple"
)

func (h *APIHandler) discardAppleAuthorization(ctx context.Context, refreshToken string) {
	if refreshToken == "" {
		return
	}
	if h.AppleCredentials != nil {
		queueCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err := h.AppleCredentials.QueueRevocation(queueCtx, refreshToken)
		cancel()
		if err == nil {
			return
		}
		if h.Logger != nil {
			h.Logger.Error("could not queue Apple authorization revocation", "err", err)
		}
	}
	if h.Apple != nil {
		revokeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err := h.Apple.Revoke(revokeCtx, refreshToken)
		cancel()
		if err != nil && h.Logger != nil {
			h.Logger.Error("could not revoke unused Apple authorization", "err", err)
		}
	}
}

func (h *APIHandler) appleProvider() (oauth.OAuthProvider, bool) {
	for _, provider := range h.OAuthProviders.Providers {
		if provider.Name == domain.ProviderApple && provider.ClientID != "" && provider.RedirectURI != "" {
			return provider, true
		}
	}
	return oauth.OAuthProvider{}, false
}

func (h *APIHandler) verifyAppleAuthorization(
	ctx context.Context,
	r *http.Request,
	code string,
	state string,
) (apple.ExchangeResult, http.Header, response.ErrorCode, string, bool) {
	if _, ok := h.appleProvider(); !ok || h.Apple == nil {
		return apple.ExchangeResult{}, nil, response.CodeNotFound, "Apple login is not enabled.", false
	}
	code = strings.TrimSpace(code)
	state = strings.TrimSpace(state)
	if code == "" || state == "" {
		return apple.ExchangeResult{}, nil, response.CodeInvalidRequest, "Apple authorization code and state required.", false
	}
	header := make(http.Header)
	flow, ok := applestate.Consume(contract.HeaderWriter(header), r, state)
	if !ok {
		return apple.ExchangeResult{}, nil, response.CodeUnauthorized, "Invalid Apple authorization.", false
	}
	result, err := h.Apple.Exchange(ctx, code, flow.Nonce)
	if err != nil || result.Identity.OAuthID == "" {
		return apple.ExchangeResult{}, header, response.CodeUnauthorized, "Invalid Apple authorization.", false
	}
	return result, header, "", "", true
}

type loginWithAppleHeadersResponse struct {
	header   http.Header
	response contract.LoginWithAppleResponseObject
}

func (r loginWithAppleHeadersResponse) VisitLoginWithAppleResponse(w http.ResponseWriter) error {
	copyResponseHeaders(w.Header(), r.header)
	return r.response.VisitLoginWithAppleResponse(w)
}

type linkCurrentUserAppleHeadersResponse struct {
	header   http.Header
	response contract.LinkCurrentUserAppleResponseObject
}

func (r linkCurrentUserAppleHeadersResponse) VisitLinkCurrentUserAppleResponse(w http.ResponseWriter) error {
	copyResponseHeaders(w.Header(), r.header)
	return r.response.VisitLinkCurrentUserAppleResponse(w)
}

type reauthenticateWithAppleHeadersResponse struct {
	header   http.Header
	response contract.ReauthenticateWithAppleResponseObject
}

type completeAccountRecoveryLinkWithAppleHeadersResponse struct {
	header   http.Header
	response contract.CompleteAccountRecoveryLinkWithAppleResponseObject
}

func (r completeAccountRecoveryLinkWithAppleHeadersResponse) VisitCompleteAccountRecoveryLinkWithAppleResponse(w http.ResponseWriter) error {
	copyResponseHeaders(w.Header(), r.header)
	return r.response.VisitCompleteAccountRecoveryLinkWithAppleResponse(w)
}

func (r reauthenticateWithAppleHeadersResponse) VisitReauthenticateWithAppleResponse(w http.ResponseWriter) error {
	copyResponseHeaders(w.Header(), r.header)
	return r.response.VisitReauthenticateWithAppleResponse(w)
}

func copyResponseHeaders(destination http.Header, source http.Header) {
	for name, values := range source {
		for _, value := range values {
			destination.Add(name, value)
		}
	}
}

func appleLoginErrorWithHeaders(code response.ErrorCode, message string, header http.Header) contract.LoginWithAppleResponseObject {
	return loginWithAppleHeadersResponse{header: header, response: loginWithAppleError(code, message)}
}

func appleLinkErrorWithHeaders(code response.ErrorCode, message string, header http.Header) contract.LinkCurrentUserAppleResponseObject {
	return linkCurrentUserAppleHeadersResponse{header: header, response: linkCurrentUserAppleError(code, message)}
}

func appleReauthenticationErrorWithHeaders(code response.ErrorCode, message string, header http.Header) contract.ReauthenticateWithAppleResponseObject {
	return reauthenticateWithAppleHeadersResponse{header: header, response: reauthenticateWithAppleError(code, message)}
}

func appleRecoveryErrorWithHeaders(code response.ErrorCode, message string, header http.Header) contract.CompleteAccountRecoveryLinkWithAppleResponseObject {
	return completeAccountRecoveryLinkWithAppleHeadersResponse{header: header, response: completeAccountRecoveryLinkWithAppleError(code, message)}
}
