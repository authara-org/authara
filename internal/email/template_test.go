package email

import (
	"errors"
	"strings"
	"testing"
)

func TestBuildOrganizationInvitationMessageRendersTemplate(t *testing.T) {
	msg, err := BuildOrganizationInvitationMessage(OrganizationInvitationPayload{
		OrganizationName: "Acme <Team>",
		InviteURL:        "https://authara.example/auth/invitations/accept?token=abc",
		Role:             "admin",
		ExpiresAt:        "2026-06-24T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("BuildOrganizationInvitationMessage failed: %v", err)
	}

	if !strings.Contains(msg.Text, "Accept the invitation: https://authara.example/auth/invitations/accept?token=abc") {
		t.Fatalf("expected invite URL in text body, got: %s", msg.Text)
	}
	if strings.Contains(msg.HTML, "Acme <Team>") || !strings.Contains(msg.HTML, "Acme &lt;Team&gt;") {
		t.Fatalf("expected escaped organization name in HTML, got: %s", msg.HTML)
	}
	if !strings.Contains(msg.HTML, "Accept invitation") {
		t.Fatalf("expected invitation button in HTML, got: %s", msg.HTML)
	}
}

func TestBuildOrganizationInvitationMessageRejectsBlankOrganizationName(t *testing.T) {
	_, err := BuildOrganizationInvitationMessage(OrganizationInvitationPayload{
		OrganizationName: " ",
		InviteURL:        "https://authara.example/auth/invitations/accept?token=abc",
		Role:             "admin",
		ExpiresAt:        "2026-06-24T12:00:00Z",
	})
	if !errors.Is(err, ErrInvalidOrganizationName) {
		t.Fatalf("expected ErrInvalidOrganizationName, got %v", err)
	}
}
