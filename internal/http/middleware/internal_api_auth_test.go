package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireInternalAPIAuth(t *testing.T) {
	for _, tc := range []struct {
		name   string
		token  string
		header string
		want   int
	}{
		{name: "valid", token: "secret", header: "Bearer secret", want: http.StatusNoContent},
		{name: "missing", token: "secret", want: http.StatusUnauthorized},
		{name: "wrong", token: "secret", header: "Bearer nope", want: http.StatusUnauthorized},
		{name: "disabled", token: "", header: "Bearer secret", want: http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/auth/internal/v1/capabilities", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rr := httptest.NewRecorder()

			RequireInternalAPIAuth(tc.token)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})).ServeHTTP(rr, req)

			if rr.Code != tc.want {
				t.Fatalf("expected status %d, got %d", tc.want, rr.Code)
			}
		})
	}
}
