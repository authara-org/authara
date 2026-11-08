package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/authara-org/authara/internal/logging"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-ID"

// RequestID accepts only canonical UUID v4/v7 correlation values and replaces
// every missing or malformed value before it can enter application context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get(requestIDHeader))
		if !logging.ValidRequestID(requestID) {
			requestID = uuid.NewString()
		}
		ctx := context.WithValue(r.Context(), chimw.RequestIDKey, requestID)
		ctx = logging.WithRequestID(ctx, requestID)
		w.Header().Set(requestIDHeader, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
