package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/render"
)

func TestSignupPostRejectsAdminAudienceBeforeSignup(t *testing.T) {
	h := &UIHandler{
		Render: render.New(render.Assets{}, false),
	}

	form := url.Values{}
	form.Set("email", "new-user@example.com")
	form.Set("password", "password123")

	req := httptest.NewRequest(http.MethodPost, "/auth/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(httpctx.WithReturnTo(req.Context(), "/admin"))
	rr := httptest.NewRecorder()

	h.SignupPost(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusForbidden, rr.Code, rr.Body.String())
	}
}
