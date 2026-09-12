package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
)

func TestReturnToWithPolicyReadsCurrentDefaultForEveryRequest(t *testing.T) {
	current := "/first"
	middleware := ReturnToWithPolicy(config.UIPolicyReaderFunc(func() config.UIPolicy {
		return config.UIPolicy{DefaultReturnTo: current}
	}))
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(httpctx.ReturnToOrDefault(r.Context())))
	}))

	request := func() string {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/auth/login", nil))
		return recorder.Body.String()
	}

	if got := request(); got != "/first" {
		t.Fatalf("first default return path = %q", got)
	}
	current = "/second"
	if got := request(); got != "/second" {
		t.Fatalf("updated default return path = %q", got)
	}
}
