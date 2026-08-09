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
