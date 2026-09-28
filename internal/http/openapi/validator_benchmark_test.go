package openapi

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkValidationMiddleware(b *testing.B) {
	for _, size := range []int{1 << 10, 1 << 20} {
		body := append([]byte(`{"csrf_token":"`), bytes.Repeat([]byte("a"), size)...)
		body = append(body, []byte(`"}`)...)
		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
		})
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		benchmarks := []struct {
			name    string
			handler http.Handler
		}{
			{name: "direct", handler: handler},
			{name: "production_request_validation", handler: ValidationMiddleware(logger, false)(handler)},
			{name: "strict_response_validation", handler: ValidationMiddleware(logger, true)(handler)},
		}
		for _, benchmark := range benchmarks {
			b.Run(fmt.Sprintf("%s/%d", benchmark.name, size), func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(body)))
				for range b.N {
					req := httptest.NewRequest(http.MethodGet, "/auth/api/v1/csrf", nil)
					rr := httptest.NewRecorder()
					benchmark.handler.ServeHTTP(rr, req)
				}
			})
		}
	}
}
