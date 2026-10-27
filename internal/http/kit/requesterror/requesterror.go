package requesterror

import (
	"context"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/http/kit/htmx"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/http/templates"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
)

type pageURLResolverKey struct{}

func CustomPages(resolve func(int) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), pageURLResolverKey{}, resolve)))
		})
	}
}

func Render(renderer render.Renderer, w http.ResponseWriter, r *http.Request, status int, message string) error {
	status = normalizeStatus(status)
	message = normalizeMessage(status, message)

	if isPartialRequest(r) {
		return Toast(renderer, w, r, status, message)
	}
	if target := customPageURL(r, status); target != "" {
		http.Redirect(w, r, target, http.StatusSeeOther)
		return nil
	}

	return renderWith(renderer, w, r, status, templates.ErrorPage(status, message))
}

func Toast(renderer render.Renderer, w http.ResponseWriter, r *http.Request, status int, message string) error {
	status = normalizeStatus(status)
	message = normalizeMessage(status, message)
	htmx.ReSwap(w, "none")
	return renderWith(renderer, w, r, status, toast.ToastMessage(toast.Error, message))
}

func NotFound(renderer render.Renderer, w http.ResponseWriter, r *http.Request) error {
	return Render(renderer, w, r, http.StatusNotFound, "Page not found.")
}

func BadRequest(renderer render.Renderer, w http.ResponseWriter, r *http.Request) error {
	return Render(renderer, w, r, http.StatusBadRequest, "Invalid request.")
}

func Unauthorized(renderer render.Renderer, w http.ResponseWriter, r *http.Request) error {
	return Render(renderer, w, r, http.StatusUnauthorized, "Please sign in again.")
}

func Forbidden(renderer render.Renderer, w http.ResponseWriter, r *http.Request) error {
	return Render(renderer, w, r, http.StatusForbidden, "You do not have access to this page.")
}

func Internal(renderer render.Renderer, w http.ResponseWriter, r *http.Request) error {
	return Render(renderer, w, r, http.StatusInternalServerError, "Something went wrong. Please try again.")
}

func renderWith(renderer render.Renderer, w http.ResponseWriter, r *http.Request, status int, component templ.Component) error {
	if renderer != nil {
		return renderer(w, r, status, component)
	}

	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	w.WriteHeader(status)
	return component.Render(r.Context(), w)
}

func normalizeStatus(status int) int {
	if status < http.StatusBadRequest {
		return http.StatusInternalServerError
	}
	return status
}

func normalizeMessage(status int, message string) string {
	if message != "" {
		return message
	}

	switch status {
	case http.StatusBadRequest:
		return "Invalid request."
	case http.StatusUnauthorized:
		return "Please sign in again."
	case http.StatusForbidden:
		return "You do not have access to this page."
	case http.StatusNotFound:
		return "Page not found."
	default:
		return "Something went wrong. Please try again."
	}
}

func isPartialRequest(r *http.Request) bool {
	return httpctx.IsHTMX(r.Context()) || r.Header.Get("HX-Request") == "true"
}

func customPageURL(r *http.Request, status int) string {
	resolve, _ := r.Context().Value(pageURLResolverKey{}).(func(int) string)
	if resolve == nil {
		return ""
	}
	return strings.TrimSpace(resolve(status))
}
