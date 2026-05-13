package redirect

import "testing"

func TestNormalizeReturnToRejectsAmbiguousRedirectTargets(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "protocol relative", raw: "//evil.example"},
		{name: "absolute url", raw: "https://evil.example"},
		{name: "leading whitespace", raw: " /account"},
		{name: "trailing whitespace", raw: "/account "},
		{name: "tab", raw: "/account\t"},
		{name: "newline", raw: "/account\n"},
		{name: "backslash after slash", raw: `/\evil.example`},
		{name: "embedded backslash", raw: `/account\settings`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, ok := NormalizeReturnTo(tt.raw); ok || got != "" {
				t.Fatalf("expected %q to be rejected, got %q ok=%v", tt.raw, got, ok)
			}
		})
	}
}

func TestNormalizeReturnToAllowsInternalRelativePaths(t *testing.T) {
	got, ok := NormalizeReturnTo("/account?tab=sessions#current")
	if !ok {
		t.Fatal("expected internal relative path to be accepted")
	}
	if got != "/account?tab=sessions#current" {
		t.Fatalf("unexpected normalized path %q", got)
	}
}
