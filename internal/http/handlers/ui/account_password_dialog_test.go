package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/http/kit/render"
	userview "github.com/authara-org/authara/internal/http/templates/user"
)

func TestRenderAccountPasswordDialogSuccessUpdatesMethodsAndClosesDialog(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/providers/password/change", nil)
	req.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()

	renderAccountPasswordDialogSuccess(
		render.New(render.Assets{}, false),
		rr,
		req,
		userview.AccountConfig{},
		"Password updated.",
	)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	for header, want := range map[string]string{
		"HX-Retarget":                     "#linked-providers-section",
		"HX-Reswap":                       "outerHTML",
		"X-Authara-Close-Password-Dialog": "true",
	} {
		if got := rr.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	for _, want := range []string{`id="linked-providers-section"`, "Password updated."} {
		if !strings.Contains(rr.Body.String(), want) {
			t.Errorf("response does not contain %q", want)
		}
	}
}
