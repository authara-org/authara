package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/authara-org/authara/internal/http/kit/render"
)

func TestHostedAccountDeletionIsDisabledByDefault(t *testing.T) {
	h := &UIHandler{Render: render.New(render.Assets{}, false)}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/user/delete", nil)

	h.DeleteUser(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}
