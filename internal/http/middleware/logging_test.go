package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/logging"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

func requestLogTestLogger(t *testing.T, output *bytes.Buffer) *logging.Logger {
	t.Helper()
	logger, err := logging.New(logging.Config{
		Level: "debug", Environment: "test", Version: "test",
		InstanceID: "00000000-0000-4000-8000-000000000001", Writer: output,
	})
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	return logger
}

func TestRequestLoggerUsesRouteTemplateAndExcludesRequestData(t *testing.T) {
	var output bytes.Buffer
	router := chi.NewRouter()
	router.Use(RequestID)
	router.Use(RequestLogger(requestLogTestLogger(t, &output)))
	router.Get("/users/{userID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/users/alice@example.com?token=secret-token", nil)
	request.Header.Set(requestIDHeader, "customer-supplied-correlation")
	request.Header.Set("User-Agent", "private-browser-fingerprint")
	request.Header.Set("X-Forwarded-For", "203.0.113.9")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	requestID := response.Header().Get(requestIDHeader)
	if !logging.ValidRequestID(requestID) {
		t.Fatalf("response request ID = %q", requestID)
	}
	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &record); err != nil {
		t.Fatalf("log JSON: %v: %q", err, output.String())
	}
	if record["event"] != "request_completed" || record["route"] != "/users/{userID}" || record["request_id"] != requestID {
		t.Fatalf("request record = %#v", record)
	}
	for _, secret := range []string{"alice@example.com", "secret-token", "private-browser-fingerprint", "203.0.113.9", "customer-supplied-correlation"} {
		if strings.Contains(output.String(), secret) {
			t.Errorf("request log contains %q", secret)
		}
	}
}

func TestRequestIDPreservesCanonicalUUID(t *testing.T) {
	want := uuid.NewString()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(requestIDHeader, want)
	response := httptest.NewRecorder()
	RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := chimw.GetReqID(r.Context()); got != want {
			t.Fatalf("context request ID = %q", got)
		}
	})).ServeHTTP(response, request)
	if got := response.Header().Get(requestIDHeader); got != want {
		t.Fatalf("response request ID = %q, want %q", got, want)
	}
}

func TestRecovererDoesNotLogPanicValueOrStack(t *testing.T) {
	var output bytes.Buffer
	logger := requestLogTestLogger(t, &output)
	handler := RequestID(Recoverer(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("customer@example.com secret-token")
	})))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(output.String(), `"event":"internal_operation"`) || !strings.Contains(output.String(), `"error_code":"internal_failure"`) {
		t.Fatalf("panic log = %q", output.String())
	}
	for _, forbidden := range []string{"customer@example.com", "secret-token", "goroutine", "panic"} {
		if strings.Contains(output.String(), forbidden) {
			t.Errorf("panic output contains %q", forbidden)
		}
	}
}
