package operator

import (
	"context"
	htmlpkg "html"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/google/uuid"
)

func TestDashboardLinksOperatorAndAccountPages(t *testing.T) {
	html := renderOperatorComponent(t, Dashboard(len(email.TemplateCatalog())))

	for _, want := range []string{
		"Operator workspace ready",
		"/auth/operator/emails",
		"/auth/operator/settings",
		"/auth/account?return_to=/auth/operator",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected dashboard to contain %q", want)
		}
	}
}

func TestRuntimeSettingsRenderSourcesLocksAndResetControls(t *testing.T) {
	dormant := "45m0s"
	html := renderOperatorComponent(t, Settings(SettingsPageModel{Settings: []config.Description{
		{
			Definition: config.Definition{
				Key: config.KeyChallengeTTL, Name: "Challenge lifetime", Description: "Maximum lifetime.",
				Environment: "AUTHARA_CHALLENGE_TTL", Reload: config.ReloadDynamic,
				Group: "Challenge", Type: config.TypeDuration, Minimum: "5m", Maximum: "24h", Impact: "New challenges only.",
			},
			EffectiveValue: "1h0m0s", EffectiveSource: config.SourceEnvironment,
			Locked: true, PersistedOverride: &dormant, Revision: 4, DormantOverride: true,
		},
		{
			Definition: config.Definition{
				Key: config.KeyChallengeMaxAttempts, Name: "Maximum attempts", Description: "Attempt limit.",
				Environment: "AUTHARA_CHALLENGE_MAX_ATTEMPTS", Reload: config.ReloadDynamic,
				Group: "Challenge", Type: config.TypeInt, Minimum: "1", Maximum: "20", Impact: "New challenges only.",
			},
			EffectiveValue: "8", EffectiveSource: config.SourceOperator, Revision: 7,
		},
	}}))

	for _, want := range []string{
		"Environment and runtime settings", "Challenge", "Set · environment", "Deployment only", "Dormant operator override: 45m0s",
		`action="/auth/operator/settings/challenge.ttl/clear"`, "Clear dormant override",
		"Set · operator override", "Editable here", `action="/auth/operator/settings/challenge.max_attempts"`,
		`action="/auth/operator/settings/challenge.max_attempts/clear"`, "Reset to default",
		"Every environment variable supported by Core is listed below.", "1 live",
		`hx-post="/auth/operator/settings/challenge.max_attempts"`, `hx-target="closest article"`,
		`hx-swap="outerHTML"`, `hx-sync="closest article:replace"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("runtime settings page does not contain %q", want)
		}
	}
	for _, action := range []string{`action="/auth/operator/settings/challenge.ttl"`} {
		if strings.Contains(html, action) {
			t.Fatalf("environment-managed setting rendered form %q", action)
		}
	}
}

func TestRuntimeSettingsPrioritizesGroupsWithLiveValues(t *testing.T) {
	html := renderOperatorComponent(t, Settings(SettingsPageModel{Settings: []config.Description{
		{Definition: config.Definition{Key: "APP_ENV", Environment: "APP_ENV", Group: "Runtime"}, Locked: true},
		{Definition: config.Definition{Key: config.KeyRateLimitLoginIPLimit, Environment: "AUTHARA_RATE_LIMIT_LOGIN_IP_LIMIT", Group: "Rate limits"}},
		{Definition: config.Definition{Key: "PUBLIC_URL", Environment: "PUBLIC_URL", Group: "Public URL"}, Locked: true},
	}}))

	rateLimits := strings.Index(html, ">Rate limits<")
	runtime := strings.Index(html, ">Runtime<")
	if rateLimits < 0 || runtime < 0 || rateLimits >= runtime {
		t.Fatalf("live-editable group was not rendered before deployment-only groups: %s", html)
	}
	if !strings.Contains(html, "1 live") {
		t.Fatal("live-editable group does not show its live-setting count")
	}
}

func TestRuntimeSettingsRenderGroupsStatesMutabilityAndHideSecrets(t *testing.T) {
	html := renderOperatorComponent(t, Settings(SettingsPageModel{Settings: []config.Description{
		{
			Definition:     config.Definition{Key: "APP_ENV", Name: "Application environment", Environment: "APP_ENV", Group: "Application", Type: config.TypeEnum, HasDefault: true},
			EffectiveValue: "dev", EffectiveSource: config.SourceDefault, Locked: true,
		},
		{
			Definition:     config.Definition{Key: "PUBLIC_URL", Name: "Public URL", Environment: "PUBLIC_URL", Group: "Application", Type: config.TypeURL, Required: true},
			EffectiveValue: "https://auth.example", EffectiveSource: config.SourceEnvironment, Locked: true,
		},
		{
			Definition:     config.Definition{Key: "AUTHARA_JWT_KEYS", Name: "JWT keys", Environment: "AUTHARA_JWT_KEYS", Group: "Token", Type: config.TypeMap, Required: true, Sensitive: true},
			EffectiveValue: "must-never-render", EffectiveSource: config.SourceEnvironment, Locked: true,
		},
		{
			Definition:      config.Definition{Key: "AUTHARA_OAUTH_GOOGLE_CLIENT_ID", Name: "Google client ID", Environment: "AUTHARA_OAUTH_GOOGLE_CLIENT_ID", Group: "OAuth", Type: config.TypeString},
			EffectiveSource: config.SourceUnset, Locked: true,
		},
	}}))

	for _, want := range []string{
		"Application", "Token", "OAuth", "Not set · default used", "Not set", "Set · environment",
		"Deployment only", "Required", "Optional", "https://auth.example", "Configured (value hidden)",
		"No value is configured and there is no built-in default.", "1 variable", "2 variables",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("runtime settings page does not contain %q", want)
		}
	}
	if strings.Contains(html, "must-never-render") {
		t.Fatal("runtime settings page rendered a sensitive environment value")
	}
	if got := strings.Count(html, "<details"); got != 3 {
		t.Fatalf("runtime settings groups rendered as details = %d, want 3", got)
	}
	if strings.Contains(html, " open>") {
		t.Fatal("runtime settings groups must be closed by default")
	}
}

func TestRuntimeSettingsOpensGroupContainingValidationError(t *testing.T) {
	html := renderOperatorComponent(t, Settings(SettingsPageModel{
		Settings: []config.Description{
			{Definition: config.Definition{Key: config.KeyChallengeTTL, Environment: "AUTHARA_CHALLENGE_TTL", Group: "Challenge"}},
			{Definition: config.Definition{Key: config.KeyRateLimitLoginIPLimit, Environment: "AUTHARA_RATE_LIMIT_LOGIN_IP_LIMIT", Group: "Rate limits"}},
		},
		ErrorKey: config.KeyRateLimitLoginIPLimit,
		Error:    "must be at least 1",
	}))

	if got := strings.Count(html, " open>"); got != 1 {
		t.Fatalf("open runtime settings groups = %d, want only the group containing the error", got)
	}
}

func TestEmailTemplatesRendersSimpleUnpaginatedTable(t *testing.T) {
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
		for _, want := range []string{definition.DisplayName, definition.Description, emailTemplateHref(definition.Key), emailTemplateDeliveryHref(definition.Key)} {
			if !strings.Contains(html, htmlpkg.EscapeString(want)) {
				t.Fatalf("expected email catalog to contain %q", want)
			}
		}
		if strings.Contains(html, ">"+string(definition.Key)+"<") {
			t.Fatalf("email catalog must not visibly render template key %q", definition.Key)
		}
		for _, variable := range definition.AvailableVariables {
			if strings.Contains(html, "{{"+variable+"}}") {
				t.Fatalf("email catalog must not render available variable %q", variable)
			}
		}
	}
	for _, want := range []string{
		"<table",
		"Email template",
		"Status",
		"Delivery",
		"Action",
		"fixed inset-0 overflow-hidden",
		"flex h-full min-h-0 flex-1 flex-col gap-5 overflow-hidden",
		"overscroll-contain overflow-auto",
		"align-middle",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected email catalog table to contain %q", want)
		}
	}
	if got := strings.Count(html, `role="switch"`); got != len(definitions) {
		t.Fatalf("enabled delivery switches = %d, want %d", got, len(definitions))
	}
	for _, want := range []string{
		`aria-checked="true"`,
		`bg-blue-600`,
		`h-7 w-12`,
		`h-5 w-5`,
		`hx-target="this"`,
		`hx-swap="outerHTML"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("delivery switches do not contain %q", want)
		}
	}
	for _, unwanted := range []string{"Available variables", "Table pagination", "Page 1"} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("email catalog must not contain %q", unwanted)
		}
	}
	if strings.Contains(html, "123456") {
		t.Fatal("email catalog must not render sample template values")
	}
}

func TestEmailTemplatesRendersDisabledDeliverySwitch(t *testing.T) {
	definition, err := email.LookupTemplate(domain.EmailTemplateNewSignIn)
	if err != nil {
		t.Fatalf("LookupTemplate failed: %v", err)
	}
	html := renderOperatorComponent(t, EmailTemplates([]email.EffectiveTemplate{{
		Definition:       definition,
		Source:           email.TemplateSourceBuiltIn,
		DeliveryDisabled: true,
	}}))

	for _, want := range []string{
		`role="switch"`,
		`aria-checked="false"`,
		`name="enabled" value="true"`,
		"Delivery of New sign-in",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("disabled delivery switch does not contain %q: %s", want, html)
		}
	}
}

func TestAuditRendersOperationalMetadataWithoutTemplateSources(t *testing.T) {
	actorID := uuid.New()
	actorEmail := "operator@example.com"
	html := renderOperatorComponent(t, Audit(email.OperatorAuditPage{
		Events: []domain.OperatorAuditEvent{
			{
				CreatedAt:    time.Date(2026, time.September, 6, 13, 15, 0, 0, time.UTC),
				ActorUserID:  &actorID,
				ActorEmail:   &actorEmail,
				Action:       domain.OperatorAuditActionEmailTemplateSaved,
				ResourceType: domain.OperatorAuditResourceEmailTemplate,
				ResourceID:   string(domain.EmailTemplateSignupCode),
				Metadata:     []byte(`{"revision":2,"version":5,"html_template":"secret source"}`),
			},
			{
				CreatedAt:    time.Date(2026, time.September, 6, 13, 16, 0, 0, time.UTC),
				ActorUserID:  &actorID,
				ActorEmail:   &actorEmail,
				Action:       domain.OperatorAuditActionRuntimeSettingSet,
				ResourceType: domain.OperatorAuditResourceRuntimeSetting,
				ResourceID:   string(config.KeyChallengeTTL),
				Metadata:     []byte(`{"revision":3,"old_effective_value":"secret-like-value"}`),
			},
		},
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
		`href="/auth/operator/settings#setting-challenge.ttl"`,
		"Setting updated",
		"Challenge lifetime",
		`href="/auth/operator/audit"`,
		`data-dropdown-value="email_template.saved"`,
		`data-dropdown-value="signup_code"`,
		"Table pagination",
		"Page 1",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected audit page to contain %q", want)
		}
	}
	if strings.Contains(html, "secret source") || strings.Contains(html, "html_template") || strings.Contains(html, "secret-like-value") {
		t.Fatal("operator audit page must not render value metadata")
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
		`data-value="/auth/operator/emails/signup_code/versions/1"`,
		`name="revision" value="7"`,
		"Customized · v2",
		"Version 2",
		"Version 1",
		"2026-09-05 09:15 UTC",
		"Version 2 - 2026-09-06 10:30 UTC (current)",
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
		"Version 2 - Saved version (viewing)",
		"Version 4 - Saved version (current)",
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
