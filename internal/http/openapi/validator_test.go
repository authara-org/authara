package openapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

func serveContractRequest(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	validated := ValidationMiddleware(logger)(handler)
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
