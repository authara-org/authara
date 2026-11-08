package middleware

import (
	"net/http"

	"github.com/authara-org/authara/internal/logging"
)

// Recoverer converts panics into a bounded operational event without exposing
// the panic value or stack through stderr or the response.
func Recoverer(logger *logging.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = logging.Discard()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					if recovered == http.ErrAbortHandler {
						panic(recovered)
					}
					logger.Log(r.Context(), logging.LevelError, logging.EventInternalOperation,
						logging.ComponentField(logging.ComponentHTTP),
						logging.OperationField(logging.OperationRequest),
						logging.OutcomeField(logging.OutcomeFailed),
						logging.ErrorCodeField(logging.ErrorInternalFailure),
					)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
