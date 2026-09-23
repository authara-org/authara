package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
)

func TestEmbeddedReauthenticationUsesCompactLayout(t *testing.T) {
	var page strings.Builder
	if err := Reauthenticate(true, true, "google-client-id", "challenge-id", true).Render(context.Background(), &page); err != nil {
		t.Fatal(err)
	}

	html := page.String()
	for _, want := range []string{
		`<!doctype html>`,
		`Use one of your sign-in methods`,
		`data-passkey-reauthenticate`,
		`data-google-flow`,
		`reauthenticate`,
		`name="embedded" value="1"`,
	} {
		if !strings.Contains(strings.ToLower(html), strings.ToLower(want)) {
			t.Errorf("embedded reauthentication page does not contain %q", want)
		}
	}
	if strings.Contains(html, "min-h-screen") {
		t.Error("embedded reauthentication page must not render the full-page auth layout")
	}
}

func TestEmbeddedReauthenticationHTMXReturnsModalFragment(t *testing.T) {
	ctx := httpctx.WithHTMX(context.Background())
	var page strings.Builder
	if err := Reauthenticate(true, true, "google-client-id", "challenge-id", true).Render(ctx, &page); err != nil {
		t.Fatal(err)
	}

	html := page.String()
	for _, want := range []string{
		`Use one of your sign-in methods`,
		`hx-target="#recent-authentication-content"`,
		`name="embedded" value="1"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("modal fragment does not contain %q", want)
		}
	}
	for _, unwanted := range []string{`<!doctype html>`, `<html`, `<head`, `<body`} {
		if strings.Contains(strings.ToLower(html), unwanted) {
			t.Errorf("modal fragment contains full-document markup %q", unwanted)
		}
	}
}

func TestEmbeddedReauthenticationCompletionContainsParentSignalMarker(t *testing.T) {
	var page strings.Builder
	if err := ReauthenticationComplete(true).Render(context.Background(), &page); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page.String(), "data-recent-authentication-complete") {
		t.Error("embedded completion page is missing the parent signal marker")
	}
}
