package ui

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/htmx"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
	userview "github.com/authara-org/authara/internal/http/templates/user"
	"github.com/authara-org/authara/internal/http/viewmodel"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *UIHandler) LogoutPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	refreshToken, exists := session.ReadRefreshToken(r)
	if exists {
		_ = h.Session.Logout(ctx, refreshToken)
	}

	session.ClearSessionCookies(w)

	returnTo, ok := httpctx.ReturnTo(r.Context())
	if !ok {
		returnTo = "/"
	}

	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (h *UIHandler) RefreshPost(w http.ResponseWriter, r *http.Request) {
	now := time.Now()

	returnTo, ok := httpctx.ReturnTo(r.Context())
	if !ok {
		returnTo = r.URL.Path
		if r.URL.RawQuery != "" {
			returnTo += "?" + r.URL.RawQuery
		}
	}

	audience := redirect.AudienceForPath(returnTo)
	refresh, ok := session.ReadRefreshToken(r)
	if !ok {
		session.ClearSessionCookies(w)
		redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", returnTo), http.StatusSeeOther)
		return
	}

	accessToken, newRefreshToken, err := h.Session.RefreshSession(
		r.Context(),
		refresh,
		audience,
		now,
	)
	if err != nil {
		session.ClearSessionCookies(w)
		redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", returnTo), http.StatusSeeOther)
		return
	}

	session.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(w, newRefreshToken, int(h.RefreshTTL.Seconds()))
	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (h *UIHandler) RevokeSessionPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		htmx.ReSwap(w, "none")
		_ = h.Render(
			w,
			r,
			http.StatusUnauthorized,
			toast.ToastMessage(toast.Error, "Unauthorized."),
		)
		return
	}

	sessionIDStr := chi.URLParam(r, "sessionID")
	sessionID, err := uuid.Parse(strings.TrimSpace(sessionIDStr))
	if err != nil {
		htmx.ReSwap(w, "none")
		_ = h.Render(
			w,
			r,
			http.StatusBadRequest,
			toast.ToastMessage(toast.Error, "Invalid session."),
		)
		return
	}

	if err := h.Session.RevokeUserSession(ctx, userID, sessionID, time.Now().UTC()); err != nil {
		htmx.ReSwap(w, "none")

		msg := "Could not revoke session."
		status := http.StatusUnprocessableEntity

		switch {
		case errors.Is(err, session.ErrForbidden):
			msg = "You are not allowed to revoke this session."
		case errors.Is(err, store.ErrSessionNotFound):
			msg = "Session not found."
		default:
			h.Logger.Error("revoke session failed", "err", err)
			msg = "Something went wrong."
			status = http.StatusInternalServerError
		}

		_ = h.Render(
			w,
			r,
			status,
			toast.ToastMessage(toast.Error, msg),
		)
		return
	}

	currentSessionID, ok := httpctx.SessionID(ctx)
	if !ok {
		htmx.ReSwap(w, "none")
		_ = h.Render(
			w,
			r,
			http.StatusUnauthorized,
			toast.ToastMessage(toast.Error, "Missing current session."),
		)
		return
	}

	sessions, err := h.Session.ListUserSessions(ctx, userID, currentSessionID, time.Now().UTC())
	if err != nil {
		htmx.ReSwap(w, "none")
		h.Logger.Error("list user sessions failed", "err", err)
		_ = h.Render(
			w,
			r,
			http.StatusInternalServerError,
			toast.ToastMessage(toast.Error, "Could not load sessions."),
		)
		return
	}

	_ = h.Render(
		w,
		r,
		http.StatusOK,
		templ.Join(toast.ToastMessage(
			toast.Success, "Session revoked."),
			userview.SessionSection(toSessionViewModels(sessions, currentSessionID), currentSessionID),
		),
	)
}

func (h *UIHandler) RevokeOtherSessionsPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := httpctx.UserID(ctx)
	if !ok {
		htmx.ReSwap(w, "none")
		_ = h.Render(
			w,
			r,
			http.StatusUnauthorized,
			toast.ToastMessage(toast.Error, "Unauthorized."),
		)
		return
	}

	currentSessionID, ok := httpctx.SessionID(ctx)
	if !ok {
		htmx.ReSwap(w, "none")
		_ = h.Render(
			w,
			r,
			http.StatusUnauthorized,
			toast.ToastMessage(toast.Error, "Missing current session."),
		)
		return
	}

	if err := h.Session.RevokeOtherUserSessions(ctx, userID, currentSessionID, time.Now().UTC()); err != nil {
		htmx.ReSwap(w, "none")

		status := http.StatusUnprocessableEntity
		msg := "Could not revoke other sessions."

		if errors.Is(err, session.ErrForbidden) {
			msg = "You are not allowed to revoke these sessions."
		} else {
			h.Logger.Error("revoke other sessions failed", "err", err)
			status = http.StatusInternalServerError
			msg = "Something went wrong."
		}

		_ = h.Render(
			w,
			r,
			status,
			toast.ToastMessage(toast.Error, msg),
		)
		return
	}

	sessions, err := h.Session.ListUserSessions(ctx, userID, currentSessionID, time.Now().UTC())
	if err != nil {
		htmx.ReSwap(w, "none")
		h.Logger.Error("list user sessions failed", "err", err)
		_ = h.Render(
			w,
			r,
			http.StatusInternalServerError,
			toast.ToastMessage(toast.Error, "Could not load sessions."),
		)
		return
	}

	_ = h.Render(
		w,
		r,
		http.StatusOK,
		templ.Join(toast.ToastMessage(
			toast.Success, "All other sessions revoked."),
			userview.SessionSection(toSessionViewModels(sessions, currentSessionID), currentSessionID),
		),
	)
}

func toSessionViewModels(sessions []domain.Session, currentSessionID uuid.UUID) []viewmodel.Session {
	out := make([]viewmodel.Session, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, viewmodel.SessionFromDomain(s, currentSessionID))
	}
	return out
}
