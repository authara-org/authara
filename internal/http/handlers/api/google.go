package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/oauthstate"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/session/token"
)

type googleOptionsResponse struct {
	ClientID string `json:"client_id"`
	Nonce    string `json:"nonce"`
}

type googleLoginRequest struct {
	Credential string `json:"credential"`
	Nonce      string `json:"nonce"`
}

func (h *APIHandler) GoogleOptionsGet(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.googleClientID()
	if !ok {
		response.WriteError(
			w,
			mustRouteError(GoogleOptionsGetErrors, response.CodeNotFound),
			"Google login is not enabled.",
		)
		return
	}

	nonce, err := oauthstate.EnsureNonce(w, r)
	if err != nil {
		response.WriteError(
			w,
			mustRouteError(GoogleOptionsGetErrors, response.CodeInternalError),
			"Google login setup error.",
		)
		return
	}

	response.JSON(w, http.StatusOK, googleOptionsResponse{ClientID: clientID, Nonce: nonce})
}

func (h *APIHandler) GoogleLoginPost(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.googleClientID(); !ok || h.Google == nil {
		response.WriteError(
			w,
			mustRouteError(GoogleLoginPostErrors, response.CodeNotFound),
			"Google login is not enabled.",
		)
		return
	}

	var in googleLoginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCredentialsBodyBytes)).Decode(&in); err != nil {
		response.WriteError(
			w,
			mustRouteError(GoogleLoginPostErrors, response.CodeInvalidRequest),
			"Invalid JSON body.",
		)
		return
	}
	in.Credential = strings.TrimSpace(in.Credential)
	in.Nonce = strings.TrimSpace(in.Nonce)
	if in.Credential == "" || in.Nonce == "" {
		response.WriteError(
			w,
			mustRouteError(GoogleLoginPostErrors, response.CodeInvalidRequest),
			"Google credential and nonce required.",
		)
		return
	}

	audience, ok := readAudience(w, r, GoogleLoginPostErrors)
	if !ok {
		return
	}

	expectedNonce, ok := oauthstate.ReadNonce(r)
	if !ok || subtle.ConstantTimeCompare([]byte(in.Nonce), []byte(expectedNonce)) != 1 {
		response.WriteError(
			w,
			mustRouteError(GoogleLoginPostErrors, response.CodeUnauthorized),
			"Invalid Google credential.",
		)
		return
	}

	identity, err := h.Google.VerifyIDToken(r.Context(), in.Credential, expectedNonce)
	if err != nil {
		response.WriteError(
			w,
			mustRouteError(GoogleLoginPostErrors, response.CodeUnauthorized),
			"Invalid Google credential.",
		)
		return
	}
	oauthstate.ClearNonce(w)

	h.completeGoogleLogin(w, r, identity, audience)
}

func (h *APIHandler) completeGoogleLogin(w http.ResponseWriter, r *http.Request, identity *google.Identity, audience token.Audience) {
	user, err := h.Auth.Login(r.Context(), auth.LoginInput{
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
		response.WriteError(w, mustRouteError(GoogleLoginPostErrors, code), message)
		return
	}

	h.createSessionResponse(w, r, GoogleLoginPostErrors, user, audience, http.StatusOK)
}

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
