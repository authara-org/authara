package auth

import (
	"context"
	"strings"
	"testing"
)

func TestOrganizationSetupOffersInvitationAndCreation(t *testing.T) {
	var html strings.Builder
	if err := OrganizationSetup().Render(context.Background(), &html); err != nil {
		t.Fatal(err)
	}
	output := html.String()
	for _, want := range []string{
		"does not currently belong to an organization",
		"Accept an invitation",
		`hx-post="/auth/invitations/accept"`,
		"Create a new organization",
		`hx-post="/auth/organizations/setup"`,
	} {
		if !strings.Contains(output, want) {
			t.Errorf("organization setup page is missing %q", want)
		}
	}
}
