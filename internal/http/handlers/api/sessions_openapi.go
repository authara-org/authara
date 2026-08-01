package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
)

func (h *APIHandler) Logout(ctx context.Context, _ contract.LogoutRequestObject) (contract.LogoutResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return logoutError(responseCodeInternalError(), "API contract error."), nil
	}
	if refreshToken, exists := session.ReadRefreshToken(r); exists {
		_ = h.Session.Logout(ctx, refreshToken)
	}
	header := make(http.Header)
	session.ClearSessionCookies(contract.HeaderWriter(header))
	return contract.Logout204HeadersResponse{Header: header}, nil
}

func (h *APIHandler) RefreshSession(ctx context.Context, request contract.RefreshSessionRequestObject) (contract.RefreshSessionResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return refreshSessionError(responseCodeInternalError(), "API contract error.", nil), nil
	}
	refreshToken, ok := session.ReadRefreshToken(r)
	if !ok || refreshToken == "" {
		return refreshSessionError(responseCodeUnauthorized(), "Refresh token missing", nil), nil
	}
	audience := token.AudienceApp
	if request.Params.Audience != nil {
		audience = token.Audience(*request.Params.Audience)
	}
	_, header, code, message, ok := h.contractRefreshTokens(ctx, refreshToken, audience, true)
	if !ok {
		return refreshSessionError(code, message, header), nil
	}
	return contract.RefreshSession200HeadersResponse{Header: header}, nil
}

func (h *APIHandler) RefreshTokens(ctx context.Context, request contract.RefreshTokensRequestObject) (contract.RefreshTokensResponseObject, error) {
	if request.Body == nil {
		return refreshTokensError(responseCodeInvalidRequest(), "Invalid JSON body."), nil
	}
	refreshToken := strings.TrimSpace(request.Body.RefreshToken)
	if refreshToken == "" {
		return refreshTokensError(responseCodeInvalidRequest(), "Refresh token required."), nil
	}
	audience := token.AudienceApp
	if request.Body.Audience != nil {
		audience = token.Audience(*request.Body.Audience)
	}
	tokens, _, code, message, ok := h.contractRefreshTokens(ctx, refreshToken, audience, false)
	if !ok {
		return refreshTokensError(code, message), nil
	}
	return contract.RefreshTokens200JSONResponse(tokens), nil
}

func (h *APIHandler) contractRefreshTokens(
	ctx context.Context,
	refreshToken string,
	audience token.Audience,
	cookieBacked bool,
) (contract.Tokens, http.Header, response.ErrorCode, string, bool) {
	newAccessToken, newRefreshToken, err := h.Session.RefreshSession(ctx, refreshToken, audience, time.Now())
	header := make(http.Header)
	switch {
	case errors.Is(err, session.ErrInvalidRefreshToken),
		errors.Is(err, session.ErrRefreshTokenReuse),
		errors.Is(err, session.ErrForbidden),
		errors.Is(err, session.ErrUserDisabled),
		errors.Is(err, session.ErrUserNotAllowed):
		if cookieBacked {
			session.ClearSessionCookies(contract.HeaderWriter(header))
		}
		return contract.Tokens{}, header, response.CodeUnauthorized, "Invalid refresh token", false
	case err != nil:
		return contract.Tokens{}, header, response.CodeInternalError, "Session error", false
	}
	if cookieBacked {
		session.SetAccessToken(contract.HeaderWriter(header), newAccessToken, int(h.AccessTTL.Seconds()))
		session.SetRefreshToken(contract.HeaderWriter(header), newRefreshToken, int(h.RefreshTTL.Seconds()))
	}
	return contract.Tokens{AccessToken: newAccessToken, RefreshToken: newRefreshToken}, header, "", "", true
}
