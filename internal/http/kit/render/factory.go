package render

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/http/kit/csrf"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/oauthstate"
)

func New(a Assets, challengeEnabled bool) Renderer {
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

		_, ok = httpctx.OAuthNonce(r.Context())
		if !ok {
			nonce, err := oauthstate.EnsureNonce(w, r)
			if err != nil {
				http.Error(w, "server error", http.StatusInternalServerError)
				return err
			}
			r = r.WithContext(httpctx.WithOAuthNonce(r.Context(), nonce))
		}

		r = r.WithContext(httpctx.WithAssets(r.Context(), a))
		r = r.WithContext(httpctx.WithChallengeEnabled(r.Context(), challengeEnabled))

		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}

		w.WriteHeader(status)
		return component.Render(r.Context(), w)
	}
}
