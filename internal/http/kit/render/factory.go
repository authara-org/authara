package render

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/http/kit/csrf"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
)

func New(a Assets) Renderer {
	return func(w http.ResponseWriter, r *http.Request, status int, component templ.Component) error {
		_, ok := httpctx.CSRFToken(r.Context())
		if !ok {
			tok, err := csrf.EnsureCookie(w, r)
			if err != nil {
				http.Error(w, "server error", http.StatusInternalServerError)
				return err
			}
			r = r.WithContext(httpctx.WithCSRF(r.Context(), tok))
		}

		// Make assets available to templ via context:
		r = r.WithContext(httpctx.WithAssets(r.Context(), a))

		// (Optional but good) Ensure HTML content type
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}

		w.WriteHeader(status)
		return component.Render(r.Context(), w)
	}
}
