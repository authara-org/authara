package form

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestInputOmitsEmptyAutocomplete(t *testing.T) {
	html := renderInput(t, InputConfig{
		Name: "q",
		Type: Search,
	})

	if strings.Contains(html, `autocomplete=""`) {
		t.Fatalf("expected empty autocomplete to be omitted, got %s", html)
	}
}

func TestInputRendersAutocompleteWhenSet(t *testing.T) {
	html := renderInput(t, InputConfig{
		Name:         "email",
		Type:         Email,
		Autocomplete: "username",
	})

	if !strings.Contains(html, `autocomplete="username"`) {
		t.Fatalf("expected autocomplete to render, got %s", html)
	}
}

func renderInput(t *testing.T, cfg InputConfig) string {
	t.Helper()

	var buf bytes.Buffer
	if err := Input(cfg).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render input: %v", err)
	}
	return buf.String()
}
