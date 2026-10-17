package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/oauthstate"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/google/uuid"
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

func (h *APIHandler) verifyGoogleAuthorization(
	ctx context.Context,
	r *http.Request,
	credential *string,
	nonce *string,
	code *string,
	redirectURI *string,
	challengeIDs ...*uuid.UUID,
) (*google.Identity, http.Header, response.ErrorCode, string, bool) {
	var challengeID *uuid.UUID
	if len(challengeIDs) > 0 {
		challengeID = challengeIDs[0]
	}
	if _, ok := h.googleClientID(); !ok || h.Google == nil {
		return nil, nil, response.CodeNotFound, "Google login is not enabled.", false
	}
	credentialValue := strings.TrimSpace(optionalString(credential))
	nonceValue := strings.TrimSpace(optionalString(nonce))
	codeValue := strings.TrimSpace(optionalString(code))
	redirectURIValue := strings.TrimSpace(optionalString(redirectURI))
	legacyProvided := credentialValue != "" || nonceValue != ""
	codeProvided := codeValue != "" || redirectURIValue != ""
	if legacyProvided == codeProvided || (legacyProvided && (credentialValue == "" || nonceValue == "")) || (codeProvided && (codeValue == "" || redirectURIValue == "")) {
		return nil, nil, response.CodeInvalidRequest, "Provide either a Google authorization code and redirect_uri or a legacy credential and nonce.", false
	}

	var identity *google.Identity
	var err error
	header := make(http.Header)
	if codeProvided {
		if challengeID != nil {
			return nil, header, response.CodeInvalidRequest, "challenge_id is only valid with a Google credential and nonce.", false
		}
		identity, err = h.Google.ExchangeAuthorizationCode(ctx, codeValue, redirectURIValue)
	} else {
		expectedNonce := ""
		if challengeID != nil && h.OAuthChallenges != nil {
			challenge, challengeErr := h.OAuthChallenges.Consume(ctx, *challengeID, domain.ProviderGoogle, "", time.Now().UTC())
			if challengeErr != nil {
				if !errors.Is(challengeErr, oauth.ErrInvalidAuthenticationChallenge) {
					return nil, header, response.CodeInternalError, "Google login setup error.", false
				}
			} else {
				expectedNonce = challenge.Nonce
			}
		} else if challengeID == nil {
			expectedNonce, _ = oauthstate.Consume(contract.HeaderWriter(header), r, nonceValue)
		}
		if expectedNonce == "" || subtle.ConstantTimeCompare([]byte(nonceValue), []byte(expectedNonce)) != 1 {
			return nil, header, response.CodeUnauthorized, "Invalid Google credential.", false
		}
		identity, err = h.Google.VerifyIDToken(ctx, credentialValue, expectedNonce)
	}
	if err != nil {
		return nil, header, response.CodeUnauthorized, "Invalid Google credential.", false
	}
	return identity, header, "", "", true
}

type loginWithGoogleHeadersResponse struct {
	header   http.Header
	response contract.LoginWithGoogleResponseObject
}

func (r loginWithGoogleHeadersResponse) VisitLoginWithGoogleResponse(w http.ResponseWriter) error {
	copyResponseHeaders(w.Header(), r.header)
	return r.response.VisitLoginWithGoogleResponse(w)
}

func googleLoginErrorWithHeaders(code response.ErrorCode, message string, header http.Header, recovery ...contract.AccountRecoveryLink) contract.LoginWithGoogleResponseObject {
	return loginWithGoogleHeadersResponse{
		header:   header,
		response: loginWithGoogleError(code, message, recovery...),
	}
}
