package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/http/viewmodel"
)

func TestGoogleSigninRendersOneFullWidthGISButton(t *testing.T) {
	var html strings.Builder
	if err := GoogleSignin("Continue", "client-id", viewmodel.AuthProviderFlowReauthenticate, "").Render(context.Background(), &html); err != nil {
		t.Fatal(err)
	}

	output := html.String()
	if got := strings.Count(output, `class="g_id_signin"`); got != 1 {
		t.Fatalf("Google GIS button count = %d, want 1", got)
	}
	if !strings.Contains(output, `data-width="400"`) {
		t.Fatal("Google GIS button is missing its full-width configuration")
	}
}
