package meta

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const readinessCheckTimeout = time.Second

type ReadinessChecker interface {
	Check(context.Context) error
}

type ReadinessObserver interface {
	SetReadiness(bool)
}

func Liveness(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

type Readiness struct {
	ready    atomic.Bool
	checker  ReadinessChecker
	observer ReadinessObserver
	stateMu  sync.Mutex
}

func NewReadiness(ready bool) *Readiness {
	return NewReadinessWithChecker(ready, nil)
}

func NewReadinessWithChecker(ready bool, checker ReadinessChecker) *Readiness {
	return NewReadinessWithObserver(ready, checker, nil)
}

func NewReadinessWithObserver(ready bool, checker ReadinessChecker, observer ReadinessObserver) *Readiness {
	state := &Readiness{checker: checker, observer: observer}
	state.Set(ready)
	return state
}

func (r *Readiness) Set(ready bool) {
	if r != nil {
		r.stateMu.Lock()
		defer r.stateMu.Unlock()
		r.ready.Store(ready)
		if !ready && r.observer != nil {
			r.observer.SetReadiness(false)
		}
	}
}

func (r *Readiness) Handler(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r == nil || !r.ready.Load() || r.checker == nil {
		if r != nil && r.observer != nil {
			r.observer.SetReadiness(false)
		}
		writeUnavailable(w)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), readinessCheckTimeout)
	defer cancel()
	if err := r.checker.Check(ctx); err != nil {
		if r.observer != nil {
			r.observer.SetReadiness(false)
		}
		writeUnavailable(w)
		return
	}
	r.stateMu.Lock()
	if !r.ready.Load() {
		if r.observer != nil {
			r.observer.SetReadiness(false)
		}
		r.stateMu.Unlock()
		writeUnavailable(w)
		return
	}
	if r.observer != nil {
		r.observer.SetReadiness(true)
	}
	r.stateMu.Unlock()
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func writeUnavailable(w http.ResponseWriter) {
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(`{"status":"unavailable"}`))
}
