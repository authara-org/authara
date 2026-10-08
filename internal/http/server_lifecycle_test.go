package http

import (
	"context"
	"net"
	stdhttp "net/http"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/http/handlers/meta"
)

func TestServeTreatsShutdownAsExpected(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{
		httpServer: &stdhttp.Server{Handler: stdhttp.HandlerFunc(func(stdhttp.ResponseWriter, *stdhttp.Request) {})},
		readiness:  meta.NewReadiness(false),
	}
	result := make(chan error, 1)
	go func() { result <- server.Serve(listener) }()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("Serve returned expected-close error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not return after shutdown")
	}
}
