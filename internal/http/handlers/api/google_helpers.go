package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/oauthstate"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/oauth/google"
)

func (h *APIHandler) googleClientID() (string, bool) {
	for _, provider := range h.OAuthProviders.Providers {
		if provider.Name == domain.ProviderGoogle && provider.ClientID != "" {
			return provider.ClientID, true
		}
	}
	return "", false
}

func googleLoginErrorCode(err error) response.ErrorCode {
	switch {
	case errors.Is(err, auth.ErrAccountExistsMustLink):
		return codeAccountLinkRequired
	case errors.Is(err, auth.ErrEmailNotAllowed), errors.Is(err, auth.ErrProviderDisabled):
		return response.CodeForbidden
	default:
		return response.CodeInternalError
	}
}

func (h *APIHandler) verifyGoogleCredential(
	ctx context.Context,
	r *http.Request,
	credential string,
	nonce string,
) (*google.Identity, http.Header, response.ErrorCode, string, bool) {
	if _, ok := h.googleClientID(); !ok || h.Google == nil {
		return nil, nil, response.CodeNotFound, "Google login is not enabled.", false
	}
	credential = strings.TrimSpace(credential)
	nonce = strings.TrimSpace(nonce)
	if credential == "" || nonce == "" {
		return nil, nil, response.CodeInvalidRequest, "Google credential and nonce required.", false
	}
	expectedNonce, ok := oauthstate.ReadNonce(r)
	if !ok || subtle.ConstantTimeCompare([]byte(nonce), []byte(expectedNonce)) != 1 {
		return nil, nil, response.CodeUnauthorized, "Invalid Google credential.", false
	}
	identity, err := h.Google.VerifyIDToken(ctx, credential, expectedNonce)
	if err != nil {
		return nil, nil, response.CodeUnauthorized, "Invalid Google credential.", false
	}
	header := make(http.Header)
	oauthstate.ClearNonce(contract.HeaderWriter(header))
	return identity, header, "", "", true
}
