package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/http/kit/render"
)

func TestMicrosoftRedirectPageLoadsOnlyRedirectBridge(t *testing.T) {
	h := &UIHandler{Render: render.New(render.Assets{
		"microsoft-redirect.js": "microsoft-redirect.fingerprint.js",
	}, false)}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/auth/oauth/microsoft/redirect", nil)

	h.MicrosoftRedirectPage(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	if !strings.Contains(body, `/auth/static/microsoft-redirect.fingerprint.js`) {
		t.Fatalf("redirect bridge asset missing from %q", body)
	}
	if strings.Contains(body, `/auth/static/app.js`) {
		t.Fatalf("application bundle must not load on redirect page: %q", body)
	}
}
