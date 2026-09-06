package challenge

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/store"
)

func TestWorkerRendersInvitationAtDeliveryTime(t *testing.T) {
	templateData := email.TemplateData{
		email.TemplateVariableOrganizationName: "Acme",
		email.TemplateVariableInviteURL:        "https://authara.example/auth/invitations/accept?token=abc",
		email.TemplateVariableInvitationCode:   "invite-123",
		email.TemplateVariableRole:             "member",
		email.TemplateVariableExpiresAt:        "2026-09-07T12:00:00Z",
	}
	rawTemplateData, err := json.Marshal(templateData)
	if err != nil {
		t.Fatalf("marshal template data: %v", err)
	}

	templateStore := &currentTemplateStore{}
	renderer := email.NewTemplateService(templateStore)
	sender := &recordingEmailSender{}
	worker := NewWorker(nil, nil, renderer, sender, nil, WorkerConfig{})
	job := domain.EmailJob{
		ToEmail:      "member@example.com",
		Template:     domain.EmailTemplateOrganizationInvite,
		TemplateData: rawTemplateData,
	}

	if err := worker.processJob(context.Background(), job, time.Now()); err != nil {
		t.Fatalf("process first job: %v", err)
	}
	templateStore.override = &domain.EmailTemplateOverride{
		Template:        domain.EmailTemplateOrganizationInvite,
		SubjectTemplate: "Customized invitation for {{organization_name}}",
		TextTemplate:    "Join at {{invite_url}} with {{invitation_code}} as {{role}} before {{expires_at}}.",
		HTMLTemplate:    "<p>Join at {{invite_url}} with {{invitation_code}} as {{role}} before {{expires_at}}.</p>",
		Revision:        1,
	}
	if err := worker.processJob(context.Background(), job, time.Now()); err != nil {
		t.Fatalf("process second job: %v", err)
	}

	if len(sender.messages) != 2 {
		t.Fatalf("sent messages = %d, want 2", len(sender.messages))
	}
	if sender.messages[0].Subject != "You're invited to Acme" || sender.messages[1].Subject != "Customized invitation for Acme" {
		t.Fatalf("sent subjects = %q, %q", sender.messages[0].Subject, sender.messages[1].Subject)
	}
	if sender.messages[1].Text != "Join at https://authara.example/auth/invitations/accept?token=abc with invite-123 as member before 2026-09-07T12:00:00Z." {
		t.Fatalf("customized text = %q", sender.messages[1].Text)
	}
}

func TestWorkerDoesNotSendWhenTemplateRenderingFails(t *testing.T) {
	renderErr := errors.New("render failed")
	renderer := templateRendererFunc(func(context.Context, domain.EmailTemplate, email.TemplateData) (email.Message, error) {
		return email.Message{}, renderErr
	})
	sender := &recordingEmailSender{}
	worker := NewWorker(nil, nil, renderer, sender, nil, WorkerConfig{})
	templateData, err := json.Marshal(email.TemplateData{
		email.TemplateVariableOrganizationName: "Acme",
		email.TemplateVariableInviteURL:        "https://authara.example/invite",
		email.TemplateVariableInvitationCode:   "invite-123",
		email.TemplateVariableRole:             "member",
		email.TemplateVariableExpiresAt:        "2026-09-07T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("marshal template data: %v", err)
	}

	err = worker.processJob(context.Background(), domain.EmailJob{
		ToEmail:      "member@example.com",
		Template:     domain.EmailTemplateOrganizationInvite,
		TemplateData: templateData,
	}, time.Now())
	if !errors.Is(err, renderErr) {
		t.Fatalf("process job error = %v, want render error", err)
	}
	if len(sender.messages) != 0 {
		t.Fatalf("sent messages = %d, want 0", len(sender.messages))
	}
}

type templateRendererFunc func(context.Context, domain.EmailTemplate, email.TemplateData) (email.Message, error)

func (f templateRendererFunc) Render(ctx context.Context, key domain.EmailTemplate, data email.TemplateData) (email.Message, error) {
	return f(ctx, key, data)
}

type currentTemplateStore struct {
	email.TemplateOverrideStore
	override *domain.EmailTemplateOverride
}

func (s *currentTemplateStore) GetEmailTemplateOverride(_ context.Context, key domain.EmailTemplate) (domain.EmailTemplateOverride, error) {
	if s.override == nil || s.override.Template != key {
		return domain.EmailTemplateOverride{}, store.ErrEmailTemplateOverrideNotFound
	}
	return *s.override, nil
}

type recordingEmailSender struct {
	messages []email.Message
}

func (s *recordingEmailSender) Send(_ context.Context, _ string, message email.Message) error {
	s.messages = append(s.messages, message)
	return nil
}
