package dropdown

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDropdownRendersAccessibleListboxAndFormValue(t *testing.T) {
	html := renderDropdown(t, Config{
		ID:       "status-filter",
		Name:     "status",
		Value:    "active",
		OnChange: "handleStatus($event.detail.value)",
		Options: []Option{
			{Value: "", Label: "All statuses"},
			{Value: "active", Label: "Active"},
		},
	})

	for _, want := range []string{
		`id="status-filter"`,
		`aria-haspopup="listbox"`,
		`aria-controls="status-filter-listbox"`,
		`id="status-filter-listbox"`,
		`role="listbox"`,
		`role="option"`,
		`name="status"`,
		`value="active"`,
		`data-dropdown-value="active"`,
		`data-dropdown-label="Active"`,
		`@dropdown-change="handleStatus($event.detail.value)"`,
		`data-value="active"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dropdown does not contain %q: %s", want, html)
		}
	}
	if strings.Contains(html, "<select") || strings.Contains(html, "<option") {
		t.Fatalf("dropdown unexpectedly rendered a native select: %s", html)
	}
}

func TestDropdownFallsBackToFirstLabelForUnknownValue(t *testing.T) {
	html := renderDropdown(t, Config{
		ID:    "empty-filter",
		Value: "unknown",
		Options: []Option{
			{Value: "", Label: "Choose one"},
		},
	})

	for _, want := range []string{`data-dropdown-label="Choose one"`, `data-dropdown-value=""`} {
		if !strings.Contains(html, want) {
			t.Fatalf("dropdown did not normalize its fallback option: %s", html)
		}
	}
}

func renderDropdown(t *testing.T, cfg Config) string {
	t.Helper()

	var buf bytes.Buffer
	if err := Dropdown(cfg).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render dropdown: %v", err)
	}
	return buf.String()
}
