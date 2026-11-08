package ui

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/requesterror"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
	"github.com/authara-org/authara/internal/logging"
	"github.com/authara-org/authara/internal/ratelimiter"
)

func (h *UIHandler) logInternal(ctx context.Context, level logging.Level, component logging.Component, operation logging.Operation, outcome logging.Outcome, errorCode logging.ErrorCode) {
	if h.Logger == nil {
		return
	}
	fields := []logging.Field{
		logging.ComponentField(component),
		logging.OperationField(operation),
		logging.OutcomeField(outcome),
	}
	if errorCode != "" {
		fields = append(fields, logging.ErrorCodeField(errorCode))
	}
	h.Logger.Log(ctx, level, logging.EventInternalOperation, fields...)
}

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
			logger = logging.Discard()
		}
		logger.Error(logging.EventInternalOperation,
			logging.ComponentField(logging.ComponentAuthentication),
			logging.OperationField(logging.OperationRateLimit),
			logging.OutcomeField(logging.OutcomeFailed),
			logging.ErrorCodeField(logging.ErrorDependencyUnavailable),
		)
		return http.StatusInternalServerError, "Authentication service unavailable.", false
	}
	return 0, "", true
}
