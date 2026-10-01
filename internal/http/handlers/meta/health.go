package meta

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

const readinessCheckTimeout = time.Second

type ReadinessChecker interface {
	Ping(context.Context) error
}

func Liveness(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// Health is retained for callers that do not configure dependency-aware
// readiness. Production routes use Readiness.Handler instead.
func Health(w http.ResponseWriter, r *http.Request) {
	Liveness(w, r)
}

type Readiness struct {
	ready   atomic.Bool
	checker ReadinessChecker
}

func NewReadiness(ready bool) *Readiness {
	return NewReadinessWithChecker(ready, nil)
}

func NewReadinessWithChecker(ready bool, checker ReadinessChecker) *Readiness {
	state := &Readiness{checker: checker}
	state.Set(ready)
	return state
}

func (r *Readiness) Set(ready bool) {
	if r != nil {
		r.ready.Store(ready)
	}
}

func (r *Readiness) Handler(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r == nil || !r.ready.Load() {
		writeUnavailable(w)
		return
	}
	if r.checker != nil {
		ctx, cancel := context.WithTimeout(request.Context(), readinessCheckTimeout)
		defer cancel()
		if err := r.checker.Ping(ctx); err != nil {
			writeUnavailable(w)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func writeUnavailable(w http.ResponseWriter) {
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(`{"status":"unavailable"}`))
}
