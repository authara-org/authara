package meta

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestReadinessHandler(t *testing.T) {
	readiness := NewReadinessWithChecker(false, readinessCheckerFunc(func(context.Context) error { return nil }))
	request := httptest.NewRequest(http.MethodGet, "/auth/health", nil)

	unavailable := httptest.NewRecorder()
	readiness.Handler(unavailable, request)
	if unavailable.Code != http.StatusServiceUnavailable {
		t.Fatalf("unready status = %d, want %d", unavailable.Code, http.StatusServiceUnavailable)
	}

	readiness.Set(true)
	ready := httptest.NewRecorder()
	readiness.Handler(ready, request)
	if ready.Code != http.StatusOK {
		t.Fatalf("ready status = %d, want %d", ready.Code, http.StatusOK)
	}
}

func TestReadinessHandlerChecksDependency(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	checker := readinessCheckerFunc(func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("readiness check has no deadline")
		}
		remaining := time.Until(deadline)
		if remaining <= 0 || remaining > readinessCheckTimeout {
			t.Fatalf("readiness deadline remaining = %s", remaining)
		}
		return databaseErr
	})
	readiness := NewReadinessWithChecker(true, checker)
	request := httptest.NewRequest(http.MethodGet, "/auth/ready", nil)
	response := httptest.NewRecorder()

	readiness.Handler(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("dependency failure status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}

	databaseErr = nil
	recovered := httptest.NewRecorder()
	readiness.Handler(recovered, request)
	if recovered.Code != http.StatusOK {
		t.Fatalf("recovered dependency status = %d, want %d", recovered.Code, http.StatusOK)
	}
}

func TestReadinessHandlerPublishesEffectiveState(t *testing.T) {
	observer := &readinessObserverStub{}
	checkErr := errors.New("database unavailable")
	readiness := NewReadinessWithObserver(true, readinessCheckerFunc(func(context.Context) error {
		return checkErr
	}), observer)
	request := httptest.NewRequest(http.MethodGet, "/auth/ready", nil)

	readiness.Handler(httptest.NewRecorder(), request)
	if ready, calls := observer.state(); ready || calls != 1 {
		t.Fatalf("observer after dependency failure = ready:%t calls:%d", ready, calls)
	}

	checkErr = nil
	readiness.Handler(httptest.NewRecorder(), request)
	if ready, calls := observer.state(); !ready || calls != 2 {
		t.Fatalf("observer after recovery = ready:%t calls:%d", ready, calls)
	}

	readiness.Set(false)
	if ready, calls := observer.state(); ready || calls != 3 {
		t.Fatalf("observer after lifecycle stop = ready:%t calls:%d", ready, calls)
	}
}

func TestReadinessHandlerCannotRestoreReadinessAfterLifecycleStop(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	observer := &readinessObserverStub{}
	readiness := NewReadinessWithObserver(true, readinessCheckerFunc(func(context.Context) error {
		close(started)
		<-release
		return nil
	}), observer)
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		readiness.Handler(response, httptest.NewRequest(http.MethodGet, "/auth/ready", nil))
	}()

	<-started
	readiness.Set(false)
	close(release)
	<-done

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness response after lifecycle stop = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if ready, _ := observer.state(); ready {
		t.Fatal("in-flight readiness check restored readiness after lifecycle stop")
	}
}

func TestReadinessHandlerSkipsDependencyWhileLifecycleIsUnready(t *testing.T) {
	called := false
	readiness := NewReadinessWithChecker(false, readinessCheckerFunc(func(context.Context) error {
		called = true
		return nil
	}))
	request := httptest.NewRequest(http.MethodGet, "/auth/ready", nil)
	response := httptest.NewRecorder()

	readiness.Handler(response, request)

	if called {
		t.Fatal("dependency was checked while lifecycle was unready")
	}
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unready status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestReadinessHandlerFailsClosedWithoutChecker(t *testing.T) {
	readiness := NewReadiness(true)
	response := httptest.NewRecorder()

	readiness.Handler(response, httptest.NewRequest(http.MethodGet, "/auth/ready", nil))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness without checker status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestLivenessDoesNotCheckDependencies(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/auth/live", nil)
	response := httptest.NewRecorder()

	Liveness(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("liveness status = %d, want %d", response.Code, http.StatusOK)
	}
}

type readinessCheckerFunc func(context.Context) error

func (f readinessCheckerFunc) Check(ctx context.Context) error {
	return f(ctx)
}

type readinessObserverStub struct {
	mu    sync.Mutex
	ready bool
	calls int
}

func (o *readinessObserverStub) SetReadiness(ready bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.ready = ready
	o.calls++
}

func (o *readinessObserverStub) state() (bool, int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.ready, o.calls
}
