package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLimitRequestBodyRejectsOversizedBody(t *testing.T) {
	handler := LimitRequestBody(4)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		var maxBytesError *http.MaxBytesError
		if !errors.As(err, &maxBytesError) {
			t.Fatalf("read body error = %v, want MaxBytesError", err)
		}
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("12345"))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, req)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
}
