package meta

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReadinessHandler(t *testing.T) {
	readiness := NewReadiness(false)
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

func TestLivenessDoesNotCheckDependencies(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/auth/live", nil)
	response := httptest.NewRecorder()

	Liveness(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("liveness status = %d, want %d", response.Code, http.StatusOK)
	}
}

type readinessCheckerFunc func(context.Context) error

func (f readinessCheckerFunc) Ping(ctx context.Context) error {
	return f(ctx)
}
