package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/session"
)

func (h *APIHandler) RefreshPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	refreshToken, ok := session.ReadRefreshToken(r)
	if !ok || refreshToken == "" {
		response.ErrorJSON(
			w,
			http.StatusUnauthorized,
			response.CodeUnauthorized,
			"Refresh token missing",
		)
		return
	}

	audience, err := redirect.AudienceFromRequest(r)
	if err != nil {
		response.ErrorJSON(
			w,
			http.StatusBadRequest,
			response.CodeInvalidRequest,
			"Invalid audience",
		)

		return
	}

	now := time.Now()
	newAccessToken, newRefreshToken, err := h.Session.RefreshSession(ctx, refreshToken, audience, now)

	switch {
	case errors.Is(err, session.ErrInvalidRefreshToken),
		errors.Is(err, session.ErrForbidden):
		response.ErrorJSON(
			w,
			http.StatusUnauthorized,
			response.CodeUnauthorized,
			"Invalid refresh token",
		)
		return

	case err != nil:
		response.ErrorJSON(
			w,
			http.StatusInternalServerError,
			response.CodeInternalError,
			"Session error",
		)
		return
	}

	session.SetAccessToken(w, newAccessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(w, newRefreshToken, int(h.RefreshTTL.Seconds()))

	w.WriteHeader(http.StatusOK)
}
