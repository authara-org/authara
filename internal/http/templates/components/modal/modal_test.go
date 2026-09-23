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

func TestRecentAuthenticationDialogUsesContentSizedNativeModal(t *testing.T) {
	var dialog strings.Builder
	if err := RecentAuthenticationDialog().Render(context.Background(), &dialog); err != nil {
		t.Fatal(err)
	}
	html := dialog.String()
	for _, want := range []string{`class="native-modal`, `cursor-pointer`, `id="recent-authentication-content"`, `data-recent-authentication-loading`, `animate-pulse`, `Loading authentication`} {
		if !strings.Contains(html, want) {
			t.Errorf("recent authentication dialog does not contain %q", want)
		}
	}
	if strings.Contains(html, "h-[min(29rem") {
		t.Error("recent authentication dialog still has the old fixed content height")
	}
	if strings.Contains(html, "<iframe") {
		t.Error("recent authentication dialog must load a fragment, not a nested HTML document")
	}
}
