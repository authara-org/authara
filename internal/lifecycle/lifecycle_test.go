package lifecycle

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func TestLifecycleSIGTERM(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestLifecycleSignalHelper$")
	command.Env = append(os.Environ(), "AUTHARA_LIFECYCLE_HELPER=1")
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "ready" {
		t.Fatalf("helper readiness = %q, err = %v", scanner.Text(), scanner.Err())
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("SIGTERM helper exited unsuccessfully: %v", err)
	}
}

func TestLifecycleSignalHelper(t *testing.T) {
	if os.Getenv("AUTHARA_LIFECYCLE_HELPER") != "1" {
		return
	}
	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	server := newFakeLifecycleServer(nil)
	workers := newFakeLifecycleWorkers()
	listener := testListener(t)
	fmt.Println("ready")
	if err := superviseLifecycle(
		signalCtx, stopSignals, discardLifecycleLogger(), server, workers, listener, time.Second,
	); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestLifecycleReturnsServerFailure(t *testing.T) {
	want := errors.New("accept failed")
	server := newFakeLifecycleServer(want)
	workers := newFakeLifecycleWorkers()

	err := superviseLifecycle(
		context.Background(), func() {}, discardLifecycleLogger(), server, workers, testListener(t), time.Second,
	)
	if !errors.Is(err, want) {
		t.Fatalf("lifecycle error = %v, want server failure", err)
	}
	assertReadinessTransitions(t, server, true, false)
}

func TestLifecycleReturnsWorkerFailure(t *testing.T) {
	want := errors.New("worker invariant failed")
	server := newFakeLifecycleServer(nil)
	workers := newFakeLifecycleWorkers()
	workers.failures <- want

	err := superviseLifecycle(
		context.Background(), func() {}, discardLifecycleLogger(), server, workers, testListener(t), time.Second,
	)
	if !errors.Is(err, want) {
		t.Fatalf("lifecycle error = %v, want worker failure", err)
	}
	if !server.shutdownCalled || !workers.shutdownCalled {
		t.Fatalf("shutdown calls = (server=%t, workers=%t)", server.shutdownCalled, workers.shutdownCalled)
	}
}

func TestLifecycleUsesOneDeadline(t *testing.T) {
	signalCtx, cancel := context.WithCancel(context.Background())
	cancel()
	server := newFakeLifecycleServer(nil)
	workers := newFakeLifecycleWorkers()

	if err := superviseLifecycle(
		signalCtx, func() {}, discardLifecycleLogger(), server, workers, testListener(t), time.Second,
	); err != nil {
		t.Fatal(err)
	}
	if server.shutdownDeadline.IsZero() || workers.shutdownDeadline.IsZero() {
		t.Fatal("shutdown deadline was not propagated")
	}
	if !server.shutdownDeadline.Equal(workers.shutdownDeadline) {
		t.Fatalf("shutdown deadlines differ: server=%s workers=%s", server.shutdownDeadline, workers.shutdownDeadline)
	}
}

func TestLifecycleBoundsShutdownTimeout(t *testing.T) {
	signalCtx, cancel := context.WithCancel(context.Background())
	cancel()
	server := newFakeLifecycleServer(nil)
	server.blockShutdown = true
	workers := newFakeLifecycleWorkers()
	workers.blockShutdown = true

	started := time.Now()
	err := superviseLifecycle(
		signalCtx, func() {}, discardLifecycleLogger(), server, workers, testListener(t), 30*time.Millisecond,
	)
	if !errors.Is(err, errShutdownTimeout) {
		t.Fatalf("lifecycle error = %v, want shutdown timeout", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("shutdown took %s, want bounded completion", elapsed)
	}
	if !server.closeCalled {
		t.Fatal("server was not force-closed")
	}
}

func TestListenHTTPReportsOccupiedAddress(t *testing.T) {
	occupied := testListener(t)
	listener, err := listenHTTP(occupied.Addr().String())
	if listener != nil {
		_ = listener.Close()
		t.Fatal("second listener unexpectedly succeeded")
	}
	if err == nil || !errors.Is(err, syscall.EADDRINUSE) {
		t.Fatalf("listen error = %v, want address in use", err)
	}
}

func TestCheckHealth(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		if err := checkHealth(context.Background(), server.Client(), server.URL); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("unready", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()
		if err := checkHealth(context.Background(), server.Client(), server.URL); err == nil {
			t.Fatal("health check unexpectedly succeeded")
		}
	})
}

type fakeLifecycleServer struct {
	serveErr         error
	stopServe        chan struct{}
	shutdownCalled   bool
	shutdownDeadline time.Time
	blockShutdown    bool
	closeCalled      bool
	readiness        []bool
}

func newFakeLifecycleServer(serveErr error) *fakeLifecycleServer {
	return &fakeLifecycleServer{serveErr: serveErr, stopServe: make(chan struct{})}
}

func (s *fakeLifecycleServer) Serve(net.Listener) error {
	if s.serveErr != nil {
		return s.serveErr
	}
	<-s.stopServe
	return nil
}

func (s *fakeLifecycleServer) Shutdown(ctx context.Context) error {
	s.shutdownCalled = true
	s.shutdownDeadline, _ = ctx.Deadline()
	if s.blockShutdown {
		<-ctx.Done()
		return ctx.Err()
	}
	s.stop()
	return nil
}

func (s *fakeLifecycleServer) Close() error {
	s.closeCalled = true
	s.stop()
	return nil
}

func (s *fakeLifecycleServer) SetReady(ready bool) {
	s.readiness = append(s.readiness, ready)
}

func (s *fakeLifecycleServer) stop() {
	select {
	case <-s.stopServe:
	default:
		close(s.stopServe)
	}
}

type fakeLifecycleWorkers struct {
	failures         chan error
	shutdownCalled   bool
	shutdownDeadline time.Time
	blockShutdown    bool
}

func newFakeLifecycleWorkers() *fakeLifecycleWorkers {
	return &fakeLifecycleWorkers{failures: make(chan error, 1)}
}

func (w *fakeLifecycleWorkers) Errors() <-chan error {
	return w.failures
}

func (w *fakeLifecycleWorkers) Shutdown(ctx context.Context) error {
	w.shutdownCalled = true
	w.shutdownDeadline, _ = ctx.Deadline()
	if w.blockShutdown {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

func testListener(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return listener
}

func assertReadinessTransitions(t *testing.T, server *fakeLifecycleServer, want ...bool) {
	t.Helper()
	if len(server.readiness) != len(want) {
		t.Fatalf("readiness transitions = %v, want %v", server.readiness, want)
	}
	for i := range want {
		if server.readiness[i] != want[i] {
			t.Fatalf("readiness transitions = %v, want %v", server.readiness, want)
		}
	}
}

func discardLifecycleLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
