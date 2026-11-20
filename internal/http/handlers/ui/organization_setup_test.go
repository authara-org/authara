package ui

import (
	"testing"

	"github.com/authara-org/authara/internal/session"
	"github.com/google/uuid"
)

func TestOrganizationSetupReturnTo(t *testing.T) {
	if got := organizationSetupReturnTo(session.Credentials{}, "/app"); got != "/auth/organizations/setup?return_to=%2Fapp" {
		t.Fatalf("orgless return path = %q", got)
	}
	if got := organizationSetupReturnTo(session.Credentials{}, "/auth/invitations/accept?token=abc"); got != "/auth/invitations/accept?token=abc" {
		t.Fatalf("invitation return path = %q", got)
	}
	if got := organizationSetupReturnTo(session.Credentials{ActiveOrganizationID: uuid.New()}, "/app"); got != "/app" {
		t.Fatalf("organization-backed return path = %q", got)
	}
}
