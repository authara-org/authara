package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/logging"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func RequestLogger(logger *logging.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = logging.Discard()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/auth/static/") {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(ww, r)

			route := ""
			if rc := chi.RouteContext(r.Context()); rc != nil {
				route = rc.RoutePattern()
			}

			fields := []logging.Field{
				logging.RequestID(chimw.GetReqID(r.Context())),
				logging.Method(r.Method),
				logging.Route(route),
				logging.StatusCode(ww.status),
				logging.StatusClassField(ww.status),
				logging.ResponseSize(int64(ww.bytes)),
				logging.Duration(time.Since(start)),
			}
			if r.ContentLength >= 0 {
				fields = append(fields, logging.RequestSize(r.ContentLength))
			}

			switch {
			case ww.status >= 500:
				fields = append(fields, logging.OutcomeField(logging.OutcomeServerError))
				logger.Log(r.Context(), logging.LevelError, logging.EventRequestCompleted, fields...)
			case ww.status >= 400:
				fields = append(fields, logging.OutcomeField(logging.OutcomeClientError))
				logger.Log(r.Context(), logging.LevelWarn, logging.EventRequestCompleted, fields...)
			default:
				fields = append(fields, logging.OutcomeField(logging.OutcomeSucceeded))
				logger.Log(r.Context(), logging.LevelInfo, logging.EventRequestCompleted, fields...)
			}
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}
