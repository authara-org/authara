package middleware

import (
	"fmt"
	"net/http"

	httpcontext "github.com/alexlup06/authgate/internal/http/context"
)

func ReturnTo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rt := r.URL.Query().Get("return_to")

		fmt.Println("return to: " + rt)

		if rt != "" {
			if normalized, ok := normalizeReturnTo(rt); ok {
				r = r.WithContext(
					httpcontext.WithReturnTo(r.Context(), normalized),
				)
			}
		}

		next.ServeHTTP(w, r)
	})
}

func normalizeReturnTo(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}

	if raw[0] != '/' {
		return "", false
	}

	if len(raw) > 1 && raw[1] == '/' {
		return "", false
	}

	return raw, true
}
