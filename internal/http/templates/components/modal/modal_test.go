package modal

import (
	"context"
	"strings"
	"testing"
)

func TestRequiredPhraseConfirmation(t *testing.T) {
	open := OpenConfirm("Delete?", "Permanent.", "Delete", "delete-form", ThemeDanger, "CONFIRM")
	if !strings.Contains(open, `"requiredPhrase":"CONFIRM"`) {
		t.Fatalf("OpenConfirm() = %q, want required phrase", open)
	}

	var dialog strings.Builder
	if err := GlobalConfirmDialog().Render(context.Background(), &dialog); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`x-model="confirm.confirmation"`, `confirm.confirmation !== confirm.requiredPhrase`} {
		if !strings.Contains(dialog.String(), want) {
			t.Errorf("dialog does not contain %q", want)
		}
	}
}
