package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
)

func TestSessionHeaderDoesNotSetCookiesForTokenTransport(t *testing.T) {
	h := &APIHandler{}
	request := httptest.NewRequest(http.MethodPost, "/?session_transport=token", nil)
	header := sessionHeader(h, request, "access", "refresh")
	if cookies := header.Values("Set-Cookie"); len(cookies) != 0 {
		t.Fatalf("Set-Cookie = %v", cookies)
	}
}

func TestSessionHeaderDoesNotSetCookiesForBearerRequest(t *testing.T) {
	h := &APIHandler{}
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request = request.WithContext(httpctx.WithAuthTransport(request.Context(), httpctx.AuthTransportBearer))
	header := sessionHeader(h, request, "access", "refresh")
	if cookies := header.Values("Set-Cookie"); len(cookies) != 0 {
		t.Fatalf("Set-Cookie = %v", cookies)
	}
}

func TestSessionHeaderKeepsBrowserCookieDefault(t *testing.T) {
	h := &APIHandler{AccessTTL: 60, RefreshTTL: 120}
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	header := sessionHeader(h, request, "access", "refresh")
	if cookies := header.Values("Set-Cookie"); len(cookies) != 2 {
		t.Fatalf("Set-Cookie = %v, want access and refresh", cookies)
	}
}
