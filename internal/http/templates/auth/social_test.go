package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/viewmodel"
	"github.com/authara-org/authara/internal/oauth"
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
	if !strings.Contains(output, `dark:!border-transparent`) || !strings.Contains(output, `dark:hover:!border-transparent`) {
		t.Fatal("Google button is missing its dark-mode border override")
	}
}

func TestInvitationAppleSigninReturnsToInvitationAcceptance(t *testing.T) {
	ctx := httpctx.WithReturnTo(context.Background(), "/auth/invitations/login?token=invite-token")
	providers := []oauth.OAuthProvider{
		oauth.NewOAuthProvider(domain.ProviderGoogle, "google-client", "https://auth.example.com"),
		oauth.NewOAuthProvider(domain.ProviderApple, "apple-client", "https://auth.example.com"),
	}
	var html strings.Builder
	if err := OAuthProviders("Login", providers, "/auth/invitations/accept?token=invite-token").Render(ctx, &html); err != nil {
		t.Fatal(err)
	}
	output := html.String()
	if got := strings.Count(output, `data-return-to="/auth/invitations/login?token=invite-token"`); got != 1 {
		t.Fatal("Google invitation return path changed")
	}
	if got := strings.Count(output, `data-return-to="/auth/invitations/accept?token=invite-token"`); got != 1 {
		t.Fatal("Apple invitation does not return to acceptance")
	}
}

func TestAppleSigninRendersCustomButton(t *testing.T) {
	var html strings.Builder
	if err := AppleSignin("Continue", viewmodel.AuthProviderFlowReauthenticate, "challenge-id", "", "").Render(context.Background(), &html); err != nil {
		t.Fatal(err)
	}

	output := html.String()
	for _, want := range []string{`data-apple-button`, `Continue with Apple`, `h-[42px] w-full`, `rounded-xl`, `border-0 bg-black`, `text-base text-white`, `dark:bg-white dark:text-grey-800`, `fill="currentColor"`, `width="814" height="1000" viewBox="0 0 814 1000"`, `data-apple-flow="reauthenticate"`, `data-authentication-challenge-id="challenge-id"`} {
		if !strings.Contains(output, want) {
			t.Errorf("Apple button is missing %q", want)
		}
	}
	if strings.Contains(output, `data-apple-render-target`) {
		t.Fatal("Apple button still renders the SDK-provided visual target")
	}
	if strings.Contains(output, `data-apple-feedback`) {
		t.Fatal("Apple button still renders the deprecated inline feedback target")
	}
}

func TestGoogleButtonLabelUsesSpacedSignIn(t *testing.T) {
	for action, want := range map[string]string{
		"Signin": "Sign in with Google",
		"Login":  "Sign in with Google",
		"Signup": "Sign up with Google",
	} {
		if got := googleButtonLabel(action); got != want {
			t.Errorf("googleButtonLabel(%q) = %q, want %q", action, got, want)
		}
	}
}

func TestAppleButtonLabelMatchesAuthenticationAction(t *testing.T) {
	for action, want := range map[string]string{
		"Login":          "Sign in with Apple",
		"Signup":         "Sign up with Apple",
		"Create account": "Sign up with Apple",
		"Link":           "Link Apple",
	} {
		if got := appleButtonLabel(action); got != want {
			t.Errorf("appleButtonLabel(%q) = %q, want %q", action, got, want)
		}
	}
}
