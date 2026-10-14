package meta

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/authara-org/authara/internal/session/token"
)

func TestJWKS(t *testing.T) {
	keys, err := token.NewKeySet("key-b", map[string][]byte{
		"key-b": bytes.Repeat([]byte{2}, 32),
		"key-a": bytes.Repeat([]byte{1}, 32),
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := JWKS(keys)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/auth/.well-known/jwks.json", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Cache-Control"); got != JWKSCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", got, JWKSCacheControl)
	}
	if got := response.Header().Get("Content-Type"); got != "application/jwk-set+json" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
	etag := response.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag is empty")
	}

	var body token.JSONWebKeySet
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode JWKS: %v", err)
	}
	if len(body.Keys) != 2 || body.Keys[0].KeyID != "key-a" || body.Keys[1].KeyID != "key-b" {
		t.Fatalf("JWKS keys = %+v", body.Keys)
	}

	notModified := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/auth/.well-known/jwks.json", nil)
	request.Header.Set("If-None-Match", etag)
	handler.ServeHTTP(notModified, request)
	if notModified.Code != http.StatusNotModified || notModified.Body.Len() != 0 {
		t.Fatalf("conditional response = %d body %q", notModified.Code, notModified.Body.String())
	}
}

func TestJWKSUnavailable(t *testing.T) {
	response := httptest.NewRecorder()
	JWKS(nil).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}
