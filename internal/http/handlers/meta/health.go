package meta

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

const readinessCheckTimeout = time.Second

type ReadinessChecker interface {
	Check(context.Context) error
}

func Liveness(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
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
	if r == nil || !r.ready.Load() || r.checker == nil {
		writeUnavailable(w)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), readinessCheckTimeout)
	defer cancel()
	if err := r.checker.Check(ctx); err != nil {
		writeUnavailable(w)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func writeUnavailable(w http.ResponseWriter) {
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(`{"status":"unavailable"}`))
}
