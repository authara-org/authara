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
