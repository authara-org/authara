package render

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/alexlup06-authgate/authgate/internal/http/kit/csrf"
	"github.com/alexlup06-authgate/authgate/internal/http/kit/httpctx"
)

func Render(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	component templ.Component,
) error {

	_, ok := httpctx.CSRFToken(r.Context())
	if !ok {
		tok, err := csrf.EnsureCookie(w, r)
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return err
		}
		r = r.WithContext(httpctx.WithCSRF(r.Context(), tok))
	}

	w.WriteHeader(status)
	return component.Render(r.Context(), w)
}
