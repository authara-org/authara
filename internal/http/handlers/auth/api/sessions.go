package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/session"
)

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
	ctx := r.Context()

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

	now := time.Now()
	newAccessToken, newRefreshToken, err := h.Session.RefreshSession(ctx, refreshToken, audience, now)

	switch {
	case errors.Is(err, session.ErrInvalidRefreshToken),
		errors.Is(err, session.ErrRefreshTokenReuse),
		errors.Is(err, session.ErrForbidden),
		errors.Is(err, session.ErrUserDisabled),
		errors.Is(err, session.ErrUserNotAllowed):
		session.ClearSessionCookies(w)
		response.WriteError(
			w,
			mustRouteError(RefreshPostErrors, response.CodeUnauthorized),
			"Invalid refresh token",
		)
		return

	case err != nil:
		response.WriteError(
			w,
			mustRouteError(RefreshPostErrors, response.CodeInternalError),
			"Session error",
		)
		return
	}

	session.SetAccessToken(w, newAccessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(w, newRefreshToken, int(h.RefreshTTL.Seconds()))

	w.WriteHeader(http.StatusOK)
}
