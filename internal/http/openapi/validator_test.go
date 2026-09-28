package openapi

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestValidationMiddlewareAcceptsMatchingResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"csrf_token":"token"}`))
	})

	rr := serveContractRequest(t, handler, http.MethodGet, "/auth/api/v1/csrf", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestValidationMiddlewareRejectsUndeclaredResponseStatus(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	rr := serveContractRequest(t, handler, http.MethodGet, "/auth/api/v1/csrf", "")
	assertContractError(t, rr, http.StatusInternalServerError, "internal_error")
}

func TestValidationMiddlewareRejectsResponseShape(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})

	rr := serveContractRequest(t, handler, http.MethodGet, "/auth/api/v1/csrf", "")
	assertContractError(t, rr, http.StatusInternalServerError, "internal_error")
}

func TestValidationMiddlewareRejectsUndeclaredErrorCode(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"forbidden","message":"wrong code for this operation"}}`))
	})

	rr := serveContractRequest(t, handler, http.MethodGet, "/auth/api/v1/csrf", "")
	assertContractError(t, rr, http.StatusInternalServerError, "internal_error")
}

func TestValidationMiddlewareRejectsRequestShape(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	})

	body := `{"email":"person@example.com","password":"password123","undeclared":true}`
	rr := serveContractRequest(t, handler, http.MethodPost, "/auth/api/v1/signup/direct", body)
	assertContractError(t, rr, http.StatusBadRequest, "invalid_request")
	if called {
		t.Fatal("handler was called for a request rejected by the OpenAPI contract")
	}
}

func TestValidationMiddlewareProductionDoesNotValidateResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})

	rr := serveContractRequestMode(t, handler, false, http.MethodGet, "/auth/api/v1/csrf", "")
	if rr.Code != http.StatusOK || rr.Body.String() != `{}` {
		t.Fatalf("expected production response to pass through, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestValidationMiddlewareProductionStillRejectsInvalidRequest(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	})
	body := `{"email":"person@example.com","password":"password123","undeclared":true}`

	rr := serveContractRequestMode(t, handler, false, http.MethodPost, "/auth/api/v1/signup/direct", body)
	assertContractError(t, rr, http.StatusBadRequest, "invalid_request")
	if called {
		t.Fatal("production handler was called for a request rejected by the OpenAPI contract")
	}
}

func TestValidationMiddlewareProductionWritesLargeResponse(t *testing.T) {
	const responseSize = 1 << 20
	payload := append([]byte(`{"csrf_token":"`), bytes.Repeat([]byte("a"), responseSize)...)
	payload = append(payload, []byte(`"}`)...)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	})

	rr := serveContractRequestMode(t, handler, false, http.MethodGet, "/auth/api/v1/csrf", "")
	if rr.Code != http.StatusOK || !bytes.Equal(rr.Body.Bytes(), payload) {
		t.Fatalf("large production response was not written unchanged: status=%d bytes=%d", rr.Code, rr.Body.Len())
	}
}

func TestValidationMiddlewareProductionStreamsResponse(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "first\n")
		w.(http.Flusher).Flush()
		<-release
		_, _ = io.WriteString(w, "second\n")
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(ValidationMiddleware(logger, false)(handler))
	t.Cleanup(server.Close)
	client := server.Client()
	client.Timeout = 2 * time.Second

	res, err := client.Get(server.URL + "/auth/api/v1/csrf")
	if err != nil {
		t.Fatalf("start streaming request: %v", err)
	}
	defer res.Body.Close()
	reader := bufio.NewReader(res.Body)
	first, err := reader.ReadString('\n')
	if err != nil || first != "first\n" {
		t.Fatalf("read first streamed chunk = %q, %v", first, err)
	}

	unblock()
	second, err := reader.ReadString('\n')
	if err != nil || second != "second\n" {
		t.Fatalf("read second streamed chunk = %q, %v", second, err)
	}
}

func TestValidationMiddlewareStrictFailureIsLogged(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	validated := ValidationMiddleware(logger, true)(handler)
	rr := httptest.NewRecorder()
	validated.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/api/v1/csrf", nil))

	assertContractError(t, rr, http.StatusInternalServerError, "internal_error")
	if !strings.Contains(logs.String(), "invalid response") {
		t.Fatalf("strict validation failure was not logged: %s", logs.String())
	}
}

func serveContractRequest(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	return serveContractRequestMode(t, handler, true, method, target, body)
}

func serveContractRequestMode(t *testing.T, handler http.Handler, validateResponses bool, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	validated := ValidationMiddleware(logger, validateResponses)(handler)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	validated.ServeHTTP(rr, req)
	return rr
}

func assertContractError(t *testing.T, rr *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rr.Code != status {
		t.Fatalf("expected %d, got %d: %s", status, rr.Code, rr.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != code {
		t.Fatalf("expected error code %q, got %q", code, body.Error.Code)
	}
}
