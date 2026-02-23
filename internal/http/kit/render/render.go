package render

import (
	"net/http"

	"github.com/a-h/templ"
	httpcontext "github.com/alexlup06-authgate/authgate/internal/http/kit/context"
	"github.com/alexlup06-authgate/authgate/internal/http/kit/csrf"
)

func Render(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	component templ.Component,
) error {

	_, ok := httpcontext.CSRFToken(r.Context())
	if !ok {
		tok, err := csrf.EnsureCookie(w, r)
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return err
		}
		r = r.WithContext(httpcontext.WithCSRF(r.Context(), tok))
	}

	w.WriteHeader(status)
	return component.Render(r.Context(), w)
}
