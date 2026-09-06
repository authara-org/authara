package email

import (
	"errors"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/domain"
)

func TestRenderBuiltInOrganizationInvitationTemplate(t *testing.T) {
	msg, err := RenderBuiltInTemplate(domain.EmailTemplateOrganizationInvite, TemplateData{
		TemplateVariableOrganizationName: "Acme <Team>",
		TemplateVariableInviteURL:        "https://authara.example/auth/invitations/accept?token=abc",
		TemplateVariableInvitationCode:   "code-123",
		TemplateVariableRole:             "admin",
		TemplateVariableExpiresAt:        "2026-06-24T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("RenderBuiltInTemplate failed: %v", err)
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
	if !strings.Contains(msg.Text, "Invitation code: code-123") || !strings.Contains(msg.HTML, "code-123") {
		t.Fatalf("expected invitation code to be rendered, got text=%s html=%s", msg.Text, msg.HTML)
	}
}

func TestRenderBuiltInOrganizationInvitationRequiresInvitationCode(t *testing.T) {
	_, err := RenderBuiltInTemplate(domain.EmailTemplateOrganizationInvite, TemplateData{
		TemplateVariableOrganizationName: "Acme",
		TemplateVariableInviteURL:        "https://authara.example/auth/invitations/accept?token=abc",
		TemplateVariableRole:             "member",
		TemplateVariableExpiresAt:        "2026-06-24T12:00:00Z",
	})
	if !errors.Is(err, ErrMissingTemplateVariable) {
		t.Fatalf("expected ErrMissingTemplateVariable, got %v", err)
	}
}
