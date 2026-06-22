package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
)

const maxRefreshBodyBytes = 2048 // ponytail: refresh JSON only; raise if this endpoint grows.

type tokensResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type tokenRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
	Audience     string `json:"audience"`
}

func (h *APIHandler) LogoutPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	refreshToken, exists := session.ReadRefreshToken(r)
	if exists {
		_ = h.Session.Logout(ctx, refreshToken)
	}

	session.ClearSessionCookies(w)

	w.WriteHeader(http.StatusNoContent)
}

func (h *APIHandler) RefreshPost(w http.ResponseWriter, r *http.Request) {
	refreshToken, ok := session.ReadRefreshToken(r)
	if !ok || refreshToken == "" {
		response.WriteError(
			w,
			mustRouteError(RefreshPostErrors, response.CodeUnauthorized),
			"Refresh token missing",
		)
		return
	}

	audience, err := redirect.AudienceFromRequest(r)
	if err != nil {
		response.WriteError(
			w,
			mustRouteError(RefreshPostErrors, response.CodeInvalidRequest),
			"Invalid audience",
		)
		return
	}

	newAccessToken, newRefreshToken, ok := h.refreshSessionTokens(w, r, refreshToken, audience, true)
	if !ok {
		return
	}

	session.SetAccessToken(w, newAccessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(w, newRefreshToken, int(h.RefreshTTL.Seconds()))
	w.WriteHeader(http.StatusOK)
}

func (h *APIHandler) TokenRefreshPost(w http.ResponseWriter, r *http.Request) {
	var in tokenRefreshRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRefreshBodyBytes)).Decode(&in); err != nil {
		response.WriteError(
			w,
			mustRouteError(RefreshPostErrors, response.CodeInvalidRequest),
			"Invalid JSON body.",
		)
		return
	}

	in.RefreshToken = strings.TrimSpace(in.RefreshToken)
	if in.RefreshToken == "" {
		response.WriteError(
			w,
			mustRouteError(RefreshPostErrors, response.CodeInvalidRequest),
			"Refresh token required.",
		)
		return
	}

	audience, ok := audienceFromBody(in.Audience)
	if !ok {
		response.WriteError(
			w,
			mustRouteError(RefreshPostErrors, response.CodeInvalidRequest),
			"Invalid audience",
		)
		return
	}

	newAccessToken, newRefreshToken, ok := h.refreshSessionTokens(w, r, in.RefreshToken, audience, false)
	if !ok {
		return
	}

	response.JSON(w, http.StatusOK, tokensResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
	})
}

func (h *APIHandler) refreshSessionTokens(
	w http.ResponseWriter,
	r *http.Request,
	refreshToken string,
	audience token.Audience,
	clearCookies bool,
) (string, string, bool) {
	newAccessToken, newRefreshToken, err := h.Session.RefreshSession(r.Context(), refreshToken, audience, time.Now())

	switch {
	case errors.Is(err, session.ErrInvalidRefreshToken),
		errors.Is(err, session.ErrRefreshTokenReuse),
		errors.Is(err, session.ErrForbidden),
		errors.Is(err, session.ErrUserDisabled),
		errors.Is(err, session.ErrUserNotAllowed):
		if clearCookies {
			session.ClearSessionCookies(w)
		}
		response.WriteError(
			w,
			mustRouteError(RefreshPostErrors, response.CodeUnauthorized),
			"Invalid refresh token",
		)
		return "", "", false

	case err != nil:
		response.WriteError(
			w,
			mustRouteError(RefreshPostErrors, response.CodeInternalError),
			"Session error",
		)
		return "", "", false
	}

	return newAccessToken, newRefreshToken, true
}

func audienceFromBody(raw string) (token.Audience, bool) {
	switch strings.TrimSpace(raw) {
	case "", "app":
		return token.AudienceApp, true
	case "admin":
		return token.AudienceAdmin, true
	default:
		return "", false
	}
}
