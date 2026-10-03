package ui

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/requesterror"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
	"github.com/authara-org/authara/internal/ratelimiter"
)

func (h *UIHandler) renderFormError(w http.ResponseWriter, r *http.Request, status int, msg string, form templ.Component) {
	if !httpctx.IsHTMX(r.Context()) {
		_ = requesterror.Render(h.Render, w, r, status, msg)
		return
	}

	toastMessage := toast.ToastMessage(toast.Error, msg)

	_ = h.Render(
		w,
		r,
		status,
		templ.Join(form, toastMessage),
	)
}

func (h *UIHandler) renderRequestError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	_ = requesterror.Render(h.Render, w, r, status, msg)
}

func (h *UIHandler) renderUnauthorized(w http.ResponseWriter, r *http.Request) {
	_ = requesterror.Unauthorized(h.Render, w, r)
}

func (h *UIHandler) renderNotFound(w http.ResponseWriter, r *http.Request) {
	_ = requesterror.NotFound(h.Render, w, r)
}

func (h *UIHandler) renderInternalError(w http.ResponseWriter, r *http.Request) {
	_ = requesterror.Internal(h.Render, w, r)
}

func (h *UIHandler) rateLimitResult(allowed bool, err error, limitedMessage string) (int, string, bool) {
	if _, limited := ratelimiter.IsRateLimited(err); limited || (err == nil && !allowed) {
		return http.StatusTooManyRequests, limitedMessage, false
	}
	if err != nil {
		logger := h.Logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Error("rate limiter unavailable", "err", err)
		return http.StatusInternalServerError, "Authentication service unavailable.", false
	}
	return 0, "", true
}
