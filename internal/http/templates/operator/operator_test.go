package operator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/google/uuid"
)

func TestDashboardLinksOperatorAndAccountPages(t *testing.T) {
	html := renderOperatorComponent(t, Dashboard(len(email.TemplateCatalog())))

	for _, want := range []string{
		"Operator workspace ready",
		"/auth/operator/emails",
		"/auth/account?return_to=/auth/operator",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected dashboard to contain %q", want)
		}
	}
}

func TestEmailTemplatesRendersCatalogMetadataWithoutSampleValues(t *testing.T) {
	definitions := email.TemplateCatalog()
	templates := make([]email.EffectiveTemplate, len(definitions))
	for i, definition := range definitions {
		templates[i] = email.EffectiveTemplate{
			Definition: definition,
			Source:     email.TemplateSourceBuiltIn,
		}
	}
	html := renderOperatorComponent(t, EmailTemplates(templates))

	for _, definition := range definitions {
		for _, want := range []string{definition.DisplayName, definition.Description, string(definition.Key)} {
			if !strings.Contains(html, want) {
				t.Fatalf("expected email catalog to contain %q", want)
			}
		}
		for _, variable := range definition.AvailableVariables {
			if !strings.Contains(html, "{{"+variable+"}}") {
				t.Fatalf("expected email catalog to contain variable %q", variable)
			}
		}
	}
	if strings.Contains(html, "123456") {
		t.Fatal("email catalog must not render sample template values")
	}
}

func TestAuditRendersOperationalMetadataWithoutTemplateSources(t *testing.T) {
	actorID := uuid.New()
	actorEmail := "operator@example.com"
	html := renderOperatorComponent(t, Audit(email.OperatorAuditPage{
		Events: []domain.OperatorAuditEvent{{
			CreatedAt:    time.Date(2026, time.September, 6, 13, 15, 0, 0, time.UTC),
			ActorUserID:  &actorID,
			ActorEmail:   &actorEmail,
			Action:       domain.OperatorAuditActionEmailTemplateSaved,
			ResourceType: domain.OperatorAuditResourceEmailTemplate,
			ResourceID:   string(domain.EmailTemplateSignupCode),
			Metadata:     []byte(`{"revision":2,"version":5,"html_template":"secret source"}`),
		}},
		Page:     1,
		Size:     50,
		Action:   domain.OperatorAuditActionEmailTemplateSaved,
		Template: domain.EmailTemplateSignupCode,
	}))

	for _, want := range []string{
		"Operator audit log",
		"Template saved",
		"Signup verification",
		"operator@example.com",
		actorID.String(),
		"2026-09-06 13:15:00 UTC",
		`href="/auth/operator/emails/signup_code"`,
		`href="/auth/operator/audit"`,
		`data-dropdown-value="email_template.saved"`,
		`data-dropdown-value="signup_code"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected audit page to contain %q", want)
		}
	}
	if strings.Contains(html, "secret source") || strings.Contains(html, "html_template") {
		t.Fatal("operator audit page must not render template source metadata")
	}
}

func TestEmailTemplateEditorRendersSourcesActionsAndSandboxedPreview(t *testing.T) {
	definition, err := email.LookupTemplate(email.TemplateCatalog()[0].Key)
	if err != nil {
		t.Fatalf("LookupTemplate failed: %v", err)
	}
	preview := email.Message{
		Subject: "Preview subject",
		Text:    "Preview text",
		HTML:    `<script>window.top.location = "https://example.com"</script><p>Preview HTML</p>`,
	}
	html := renderOperatorComponent(t, EmailTemplateEditor(EmailTemplateEditorModel{
		Definition:    definition,
		Source:        email.TemplateSourceOverride,
		Revision:      7,
		ActiveVersion: 2,
		Versions: []domain.EmailTemplateVersion{
			{
				Template:        definition.Key,
				Version:         2,
				CreatedAt:       time.Date(2026, time.September, 6, 10, 30, 0, 0, time.UTC),
				SubjectTemplate: "Current subject",
			},
			{
				Template:        definition.Key,
				Version:         1,
				CreatedAt:       time.Date(2026, time.September, 5, 9, 15, 0, 0, time.UTC),
				SubjectTemplate: "Previous subject",
			},
		},
		SubjectTemplate: definition.DefaultSubjectTemplate,
		TextTemplate:    definition.DefaultTextTemplate,
		HTMLTemplate:    definition.DefaultHTMLTemplate,
		Preview:         &preview,
	}))

	for _, want := range []string{
		`action="/auth/operator/emails/signup_code"`,
		`id="email-template-selector"`,
		`data-value="/auth/operator/emails/password_reset_code"`,
		`hx-post="/auth/operator/emails/signup_code/preview"`,
		`hx-trigger="input delay:500ms"`,
		`hx-target="#email-template-preview"`,
		`hx-include="#email-template-form"`,
		`data-email-template-preview-status`,
		`data-email-template-save-request`,
		`id="email-template-actions"`,
		`id="email-template-source-status"`,
		`id="email-template-history"`,
		`id="email-template-restore-action"`,
		`id="email-template-save-action"`,
		`id="email-template-revision"`,
		`hx-target="#email-template-feedback"`,
		`hx-sync="this:abort"`,
		`hx-sync="#email-template-form:drop"`,
		`hx-disabled-elt="#email-template-actions [data-email-template-action]"`,
		`hx-disabled-elt="#email-template-actions [data-email-template-action]:not([data-email-template-save-request])"`,
		`>Render</span>`,
		`>Restore</span>`,
		`>Save</span>`,
		`action="/auth/operator/emails/signup_code/reset"`,
		`href="/auth/operator/emails/signup_code/versions/1"`,
		`name="revision" value="7"`,
		"Customized · v2",
		"Version 2",
		"Version 1",
		"2026-09-05 09:15 UTC",
		"Previous subject",
		"Current",
		`data-email-template-editor="text"`,
		`data-email-template-editor="html"`,
		`x-show="mode === 'text'"`,
		`x-show="mode === 'html'"`,
		`form="email-template-form"`,
		`sandbox=""`,
		`srcdoc=`,
		"{{code}}",
		"Preview subject",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected editor to contain %q", want)
		}
	}
	if strings.Contains(html, `<script>window.top.location`) {
		t.Fatal("preview HTML must be escaped into the sandboxed srcdoc attribute")
	}
	saveMarker := strings.Index(html, `data-email-template-save-request`)
	if saveMarker < 0 {
		t.Fatal("could not locate the save button marker")
	}
	saveButtonStart := strings.LastIndex(html[:saveMarker], "<button")
	saveButtonEnd := strings.Index(html[saveMarker:], ">")
	if saveButtonStart < 0 || saveButtonEnd < 0 {
		t.Fatal("could not locate the save button")
	}
	saveButtonTag := html[saveButtonStart : saveMarker+saveButtonEnd]
	if !strings.Contains(saveButtonTag, `type="button"`) {
		t.Fatalf("save button must not also submit the form natively: %s", saveButtonTag)
	}
}

func TestEmailTemplateEditorRendersHistoricalVersionAsUnsavedDraft(t *testing.T) {
	definition, err := email.LookupTemplate(domain.EmailTemplateSignupCode)
	if err != nil {
		t.Fatalf("LookupTemplate failed: %v", err)
	}
	model := EmailTemplateEditorModel{
		Definition:     definition,
		Source:         email.TemplateSourceOverride,
		Revision:       4,
		ActiveVersion:  4,
		ViewingVersion: 2,
		Versions: []domain.EmailTemplateVersion{
			{Template: definition.Key, Version: 4, SubjectTemplate: "Current subject"},
			{Template: definition.Key, Version: 2, SubjectTemplate: "Historical subject"},
		},
		SubjectTemplate: "Historical subject",
		TextTemplate:    "Historical {{code}}",
		HTMLTemplate:    "<p>Historical {{code}}</p>",
	}
	html := renderOperatorComponent(t, EmailTemplateEditor(model))

	for _, want := range []string{
		"Viewing v2",
		`data-email-template-viewing-version`,
		"Return to current",
		"Save as new version",
		`href="/auth/operator/emails/signup_code"`,
		`value="4"`,
		`value="Historical subject"`,
		">Viewing</span>",
		">Current</a>",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected historical editor to contain %q", want)
		}
	}
	if strings.Contains(html, "/versions/2/restore") {
		t.Fatal("historical preview must not persist through a restore route")
	}
}

func renderOperatorComponent(t *testing.T, component templ.Component) string {
	t.Helper()

	var page strings.Builder
	if err := component.Render(context.Background(), &page); err != nil {
		t.Fatalf("render operator component: %v", err)
	}
	return page.String()
}
