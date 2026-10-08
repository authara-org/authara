package auth

import (
	"context"
	"strings"
	"testing"
)

func TestLoginUsesApplicationName(t *testing.T) {
	var html strings.Builder
	if err := Login(nil, false, "Example App").Render(context.Background(), &html); err != nil {
		t.Fatal(err)
	}

	output := html.String()
	if !strings.Contains(output, "Sign in to Example App") {
		t.Fatal("login page does not render the configured application name")
	}
	if strings.Contains(output, "Welcome Back") || strings.Contains(output, "unlimited access") {
		t.Fatal("login page still renders the previous heading copy")
	}
}

func TestSignupUsesApplicationName(t *testing.T) {
	var html strings.Builder
	if err := Signup(nil, "Example App").Render(context.Background(), &html); err != nil {
		t.Fatal(err)
	}

	output := html.String()
	if !strings.Contains(output, "Create account for Example App") {
		t.Fatal("signup page does not render the configured application name")
	}
	if strings.Contains(output, "unlimited access") {
		t.Fatal("signup page still renders the previous heading copy")
	}
}
