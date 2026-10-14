package user

import (
	"context"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/http/viewmodel"
)

func TestAccountAlwaysIncludesDeletion(t *testing.T) {
	var page strings.Builder
	if err := Account(AccountConfig{}).Render(context.Background(), &page); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"Delete account...", `id="delete-account-form"`, "cursor-pointer"} {
		if !strings.Contains(page.String(), want) {
			t.Errorf("account page does not contain %q", want)
		}
	}
}

func TestAccountIncludesProviderUnlinkForms(t *testing.T) {
	var page strings.Builder
	if err := Account(AccountConfig{}).Render(context.Background(), &page); err != nil {
		t.Fatal(err)
	}

	for _, provider := range []string{"password", "google", "apple", "microsoft"} {
		for _, want := range []string{
			`id="remove-` + provider + `-form"`,
			`hx-post="/auth/providers/` + provider + `/unlink"`,
		} {
			if !strings.Contains(page.String(), want) {
				t.Errorf("account page does not contain %q", want)
			}
		}
	}
}

func TestAccountPasswordActionsOpenDialog(t *testing.T) {
	for _, test := range []struct {
		name     string
		provider viewmodel.AuthProvider
		path     string
	}{
		{
			name: "add",
			provider: viewmodel.AuthProvider{
				Kind: viewmodel.AuthProviderPassword,
			},
			path: "/auth/providers/password/add?modal=1&amp;return_to=/auth/account",
		},
		{
			name: "change",
			provider: viewmodel.AuthProvider{
				Kind:   viewmodel.AuthProviderPassword,
				Linked: true,
			},
			path: "/auth/providers/password/change?modal=1&amp;return_to=/auth/account",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var page strings.Builder
			if err := Account(AccountConfig{AuthProviders: []viewmodel.AuthProvider{test.provider}}).Render(context.Background(), &page); err != nil {
				t.Fatal(err)
			}
			html := page.String()
			for _, want := range []string{
				`id="account-password-dialog"`,
				`hx-target="#account-password-dialog-content"`,
				test.path,
			} {
				if !strings.Contains(html, want) {
					t.Errorf("account password action does not contain %q", want)
				}
			}
			if strings.Contains(html, `hx-replace-url="true"`) {
				t.Error("password dialog action must not replace the account URL")
			}
		})
	}
}

func TestGoogleLinkButtonRendersOneButton(t *testing.T) {
	var html strings.Builder
	if err := GoogleLinkButton("client-id").Render(context.Background(), &html); err != nil {
		t.Fatal(err)
	}

	output := html.String()
	if got := strings.Count(output, "<button"); got != 1 {
		t.Fatalf("Google link rendered %d buttons, want 1", got)
	}
	for _, want := range []string{`data-google-button`, `data-google-client-id="client-id"`, `Link Google`} {
		if !strings.Contains(output, want) {
			t.Errorf("Google link button is missing %q", want)
		}
	}
}

func TestAppleProviderRendersAppleIcon(t *testing.T) {
	var html strings.Builder
	if err := AuthProviderIcon(viewmodel.AuthProviderApple).Render(context.Background(), &html); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{`items-center justify-center`, `viewBox="0 0 814 1000"`} {
		if !strings.Contains(html.String(), want) {
			t.Fatalf("Apple provider icon is missing %q", want)
		}
	}
}

func TestAccountShowsOperatorWorkspaceOnlyForOperators(t *testing.T) {
	var operatorPage strings.Builder
	if err := Account(AccountConfig{OperatorAccess: true}).Render(context.Background(), &operatorPage); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(operatorPage.String(), `/auth/operator`) {
		t.Fatal("operator account page does not link to the operator workspace")
	}

	var regularPage strings.Builder
	if err := Account(AccountConfig{}).Render(context.Background(), &regularPage); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(regularPage.String(), `/auth/operator`) {
		t.Fatal("regular account page must not link to the operator workspace")
	}
}
