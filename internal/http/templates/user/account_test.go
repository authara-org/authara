package user

import (
	"context"
	"strings"
	"testing"
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
