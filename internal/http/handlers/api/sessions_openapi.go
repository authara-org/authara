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
	r, out, ok := contractRequest(ctx)
	if !ok {
		return out, nil
	}
	if refreshToken, exists := session.ReadRefreshToken(r); exists {
		_ = h.Session.Logout(ctx, refreshToken)
	}
	header := make(http.Header)
	session.ClearSessionCookies(contract.HeaderWriter(header))
	return contract.NoContent(header), nil
}

func (h *APIHandler) RefreshSession(ctx context.Context, request contract.RefreshSessionRequestObject) (contract.RefreshSessionResponseObject, error) {
	r, out, ok := contractRequest(ctx)
	if !ok {
		return out, nil
	}
	refreshToken, ok := session.ReadRefreshToken(r)
	if !ok || refreshToken == "" {
		return routeError(RefreshSessionErrors, responseCodeUnauthorized(), "Refresh token missing"), nil
	}
	audience := token.AudienceApp
	if request.Params.Audience != nil {
		audience = token.Audience(*request.Params.Audience)
	}
	_, header, out, ok := h.contractRefreshTokens(ctx, refreshToken, audience, RefreshSessionErrors, true)
	if !ok {
		return out, nil
	}
	return contract.Empty(http.StatusOK, header), nil
}

func (h *APIHandler) RefreshTokens(ctx context.Context, request contract.RefreshTokensRequestObject) (contract.RefreshTokensResponseObject, error) {
	if request.Body == nil {
		return routeError(RefreshTokensErrors, responseCodeInvalidRequest(), "Invalid JSON body."), nil
	}
	refreshToken := strings.TrimSpace(request.Body.RefreshToken)
	if refreshToken == "" {
		return routeError(RefreshTokensErrors, responseCodeInvalidRequest(), "Refresh token required."), nil
	}
	audience := token.AudienceApp
	if request.Body.Audience != nil {
		audience = token.Audience(*request.Body.Audience)
	}
	tokens, _, out, ok := h.contractRefreshTokens(ctx, refreshToken, audience, RefreshTokensErrors, false)
	if !ok {
		return out, nil
	}
	return contract.JSON(http.StatusOK, tokens), nil
}

func (h *APIHandler) contractRefreshTokens(
	ctx context.Context,
	refreshToken string,
	audience token.Audience,
	routeErrors map[response.ErrorCode]response.ErrorSpec,
	cookieBacked bool,
) (contract.Tokens, http.Header, contract.Response, bool) {
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
		out := routeError(routeErrors, response.CodeUnauthorized, "Invalid refresh token")
		out.Header = header
		return contract.Tokens{}, header, out, false
	case err != nil:
		return contract.Tokens{}, header, routeError(routeErrors, response.CodeInternalError, "Session error"), false
	}
	if cookieBacked {
		session.SetAccessToken(contract.HeaderWriter(header), newAccessToken, int(h.AccessTTL.Seconds()))
		session.SetRefreshToken(contract.HeaderWriter(header), newRefreshToken, int(h.RefreshTTL.Seconds()))
	}
	return contract.Tokens{AccessToken: newAccessToken, RefreshToken: newRefreshToken}, header, contract.Response{}, true
}
