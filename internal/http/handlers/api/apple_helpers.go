package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/applestate"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/oauth/apple"
	"github.com/google/uuid"
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
	challengeIDs ...*uuid.UUID,
) (apple.ExchangeResult, http.Header, response.ErrorCode, string, bool, bool) {
	var challengeID *uuid.UUID
	if len(challengeIDs) > 0 {
		challengeID = challengeIDs[0]
	}
	if _, ok := h.appleProvider(); !ok || h.Apple == nil {
		return apple.ExchangeResult{}, nil, response.CodeNotFound, "Apple login is not enabled.", false, false
	}
	code = strings.TrimSpace(code)
	state = strings.TrimSpace(state)
	if code == "" || state == "" {
		return apple.ExchangeResult{}, nil, response.CodeInvalidRequest, "Apple authorization code and state required.", false, false
	}
	header := make(http.Header)
	expectedNonce := ""
	if challengeID != nil && h.OAuthChallenges != nil {
		challenge, err := h.OAuthChallenges.Consume(ctx, *challengeID, domain.ProviderApple, state, time.Now().UTC())
		if err != nil && !errors.Is(err, oauth.ErrInvalidAuthenticationChallenge) {
			return apple.ExchangeResult{}, header, response.CodeInternalError, "Apple login setup error.", false, false
		}
		if err == nil {
			expectedNonce = challenge.Nonce
		}
	} else if challengeID == nil {
		flow, ok := applestate.Consume(contract.HeaderWriter(header), r, state)
		if ok {
			expectedNonce = flow.Nonce
		}
	}
	if expectedNonce == "" {
		return apple.ExchangeResult{}, nil, response.CodeUnauthorized, "Invalid Apple authorization.", false, false
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowOAuthAttempt(ctx, httputil.ClientIP(r))
		if code, message, ok := h.rateLimitResult(allowed, err, "Too many Apple authentication attempts. Please try again later."); !ok {
			if code == response.CodeRateLimited {
				return apple.ExchangeResult{}, header, response.CodeUnauthorized, "Invalid Apple authorization.", false, true
			}
			return apple.ExchangeResult{}, header, code, message, false, false
		}
	}
	client := h.Apple
	if challengeID != nil && h.AppleNative != nil {
		client = h.AppleNative
	}
	result, err := client.Exchange(ctx, code, expectedNonce)
	if err != nil || result.Identity.OAuthID == "" {
		return apple.ExchangeResult{}, header, response.CodeUnauthorized, "Invalid Apple authorization.", false, false
	}
	return result, header, "", "", true, false
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

func appleLoginErrorWithHeaders(code response.ErrorCode, message string, header http.Header, recovery ...contract.AccountRecoveryLink) contract.LoginWithAppleResponseObject {
	return loginWithAppleHeadersResponse{header: header, response: loginWithAppleError(code, message, recovery...)}
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
