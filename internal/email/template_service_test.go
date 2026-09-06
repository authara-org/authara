package email

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

func TestTemplateServiceFallsBackToBuiltInTemplate(t *testing.T) {
	service := NewTemplateService(newFakeTemplateOverrideStore())
	data := TemplateData{TemplateVariableCode: "123456"}

	effective, err := service.Get(context.Background(), domain.EmailTemplateSignupCode)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if effective.Source != TemplateSourceBuiltIn || effective.Override != nil {
		t.Fatalf("effective template = %#v, want built-in", effective)
	}

	got, err := service.Render(context.Background(), domain.EmailTemplateSignupCode, data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	want, err := RenderBuiltInTemplate(domain.EmailTemplateSignupCode, data)
	if err != nil {
		t.Fatalf("RenderBuiltInTemplate failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rendered message = %#v, want %#v", got, want)
	}
}

func TestTemplateServiceRendersPersistedOverride(t *testing.T) {
	fakeStore := newFakeTemplateOverrideStore()
	fakeStore.overrides[domain.EmailTemplateSignupCode] = domain.EmailTemplateOverride{
		Template:        domain.EmailTemplateSignupCode,
		SubjectTemplate: "Verification {{ code }}",
		TextTemplate:    "Enter {{code}}",
		HTMLTemplate:    `<p data-code="{{code}}">{{ code }}</p>`,
		Revision:        3,
	}
	service := NewTemplateService(fakeStore)

	effective, err := service.Get(context.Background(), domain.EmailTemplateSignupCode)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if effective.Source != TemplateSourceOverride || effective.Override == nil || effective.Override.Revision != 3 {
		t.Fatalf("effective template = %#v, want revision 3 override", effective)
	}

	message, err := service.Render(context.Background(), domain.EmailTemplateSignupCode, TemplateData{
		TemplateVariableCode: `<123>&`,
	})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if message.Subject != "Verification <123>&" || message.Text != "Enter <123>&" {
		t.Fatalf("plain template parts were not rendered: %#v", message)
	}
	if strings.Contains(message.HTML, `<123>`) || !strings.Contains(message.HTML, `&lt;123&gt;&amp;`) {
		t.Fatalf("HTML template data was not escaped: %s", message.HTML)
	}
}

func TestTemplateServiceSavesValidatedOverridesAndIncrementsRevision(t *testing.T) {
	fakeStore := newFakeTemplateOverrideStore()
	service := NewTemplateService(fakeStore)
	updaterID := uuid.New()
	input := SaveTemplateOverrideInput{
		Template:         domain.EmailTemplateSignupCode,
		SubjectTemplate:  "Custom verification",
		TextTemplate:     "Code: {{code}}",
		HTMLTemplate:     "<strong>{{code}}</strong>",
		ExpectedRevision: 0,
		UpdatedByUserID:  updaterID,
	}

	first, err := service.SaveOverride(context.Background(), input)
	if err != nil {
		t.Fatalf("first SaveOverride failed: %v", err)
	}
	input.ExpectedRevision = first.Override.Revision
	second, err := service.SaveOverride(context.Background(), input)
	if err != nil {
		t.Fatalf("second SaveOverride failed: %v", err)
	}
	if first.Override == nil || first.Override.Revision != 1 {
		t.Fatalf("first revision = %#v, want 1", first.Override)
	}
	if second.Override == nil || second.Override.Revision != 2 {
		t.Fatalf("second revision = %#v, want 2", second.Override)
	}
	if second.Override.UpdatedByUserID == nil || *second.Override.UpdatedByUserID != updaterID {
		t.Fatalf("updated by = %v, want %s", second.Override.UpdatedByUserID, updaterID)
	}
}

func TestTemplateServiceRejectsInvalidOverridesBeforePersistence(t *testing.T) {
	tests := []struct {
		name string
		in   SaveTemplateOverrideInput
		want error
	}{
		{
			name: "missing updater",
			in: SaveTemplateOverrideInput{
				Template:        domain.EmailTemplateSignupCode,
				SubjectTemplate: "Subject",
				TextTemplate:    "{{code}}",
				HTMLTemplate:    "<p>{{code}}</p>",
			},
			want: ErrMissingTemplateUpdater,
		},
		{
			name: "unknown variable",
			in: SaveTemplateOverrideInput{
				Template:        domain.EmailTemplateSignupCode,
				SubjectTemplate: "Subject",
				TextTemplate:    "{{unknown}}",
				HTMLTemplate:    "<p>{{code}}</p>",
				UpdatedByUserID: uuid.New(),
			},
			want: ErrUnknownTemplateVariable,
		},
		{
			name: "missing required placeholder",
			in: SaveTemplateOverrideInput{
				Template:        domain.EmailTemplateSignupCode,
				SubjectTemplate: "Subject",
				TextTemplate:    "The code was omitted",
				HTMLTemplate:    "<p>{{code}}</p>",
				UpdatedByUserID: uuid.New(),
			},
			want: ErrMissingTemplatePlaceholder,
		},
		{
			name: "malformed variable",
			in: SaveTemplateOverrideInput{
				Template:        domain.EmailTemplateSignupCode,
				SubjectTemplate: "Subject",
				TextTemplate:    "{{code",
				HTMLTemplate:    "<p>{{code}}</p>",
				UpdatedByUserID: uuid.New(),
			},
			want: ErrMalformedTemplate,
		},
		{
			name: "subject line break",
			in: SaveTemplateOverrideInput{
				Template:        domain.EmailTemplateSignupCode,
				SubjectTemplate: "First line\nSecond line",
				TextTemplate:    "{{code}}",
				HTMLTemplate:    "<p>{{code}}</p>",
				UpdatedByUserID: uuid.New(),
			},
			want: ErrInvalidTemplateSubject,
		},
		{
			name: "empty part",
			in: SaveTemplateOverrideInput{
				Template:        domain.EmailTemplateSignupCode,
				SubjectTemplate: " ",
				TextTemplate:    "{{code}}",
				HTMLTemplate:    "<p>{{code}}</p>",
				UpdatedByUserID: uuid.New(),
			},
			want: ErrEmptyTemplatePart,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeStore := newFakeTemplateOverrideStore()
			service := NewTemplateService(fakeStore)

			_, err := service.SaveOverride(context.Background(), tt.in)
			if !errors.Is(err, tt.want) {
				t.Fatalf("SaveOverride error = %v, want %v", err, tt.want)
			}
			if fakeStore.upsertCalls != 0 {
				t.Fatalf("UpsertEmailTemplateOverride called %d times", fakeStore.upsertCalls)
			}
		})
	}
}

func TestTemplateServicePreviewsIncompleteInvitationButRequiresActionPlaceholdersOnSave(t *testing.T) {
	service := NewTemplateService(newFakeTemplateOverrideStore())
	definition, err := LookupTemplate(domain.EmailTemplateOrganizationInvite)
	if err != nil {
		t.Fatalf("LookupTemplate failed: %v", err)
	}

	input := PreviewTemplateInput{
		Template:        definition.Key,
		SubjectTemplate: "Invitation for {{organization_name}}",
		TextTemplate:    "Open {{invite_url}} or enter {{invitation_code}}.",
		HTMLTemplate:    `<a href="{{invite_url}}">Accept</a><p>{{invitation_code}}</p>`,
	}
	if _, err := service.Preview(input); err != nil {
		t.Fatalf("Preview rejected a template that omits only metadata placeholders: %v", err)
	}

	input.SubjectTemplate = "Organization invitation"
	input.TextTemplate = "The invitation details are being rewritten."
	input.HTMLTemplate = `<p>The invitation details are being rewritten.</p>`
	if _, err := service.Preview(input); err != nil {
		t.Fatalf("Preview rejected an incomplete draft: %v", err)
	}

	_, err = service.SaveOverride(context.Background(), SaveTemplateOverrideInput{
		Template:         input.Template,
		SubjectTemplate:  input.SubjectTemplate,
		TextTemplate:     input.TextTemplate,
		HTMLTemplate:     input.HTMLTemplate,
		ExpectedRevision: 0,
		UpdatedByUserID:  uuid.New(),
	})
	if !errors.Is(err, ErrMissingTemplatePlaceholder) {
		t.Fatalf("SaveOverride incomplete draft error = %v, want %v", err, ErrMissingTemplatePlaceholder)
	}
}

func TestTemplateServiceReportsSourceLocationsForEditorDiagnostics(t *testing.T) {
	service := NewTemplateService(newFakeTemplateOverrideStore())
	tests := []struct {
		name     string
		textBody string
		htmlBody string
		wantPart TemplatePart
		wantLine int
	}{
		{
			name:     "malformed text placeholder",
			textBody: "First line\nSecond line\n{{code",
			htmlBody: "<p>{{code}}</p>",
			wantPart: TemplatePartText,
			wantLine: 3,
		},
		{
			name:     "unknown HTML placeholder",
			textBody: "Code: {{code}}",
			htmlBody: "<p>First line</p>\n<div>Second line</div>\n<p>{{unknown}}</p>",
			wantPart: TemplatePartHTML,
			wantLine: 3,
		},
		{
			name:     "invalid HTML context",
			textBody: "Code: {{code}}",
			htmlBody: "<!doctype html>\n<html>\n<body style=\"margin:0;\n</body>\n</html>",
			wantPart: TemplatePartHTML,
			wantLine: 3,
		},
		{
			name:     "invalid HTML attribute",
			textBody: "Code: {{code}}",
			htmlBody: "<!doctype html>\n<head>\n<meta charset=\"UTF-8\"\n</head>\n<body>{{code}}</body>",
			wantPart: TemplatePartHTML,
			wantLine: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.Preview(PreviewTemplateInput{
				Template:        domain.EmailTemplateSignupCode,
				SubjectTemplate: "Subject",
				TextTemplate:    tt.textBody,
				HTMLTemplate:    tt.htmlBody,
			})
			if err == nil {
				t.Fatal("Preview succeeded, want a template error")
			}
			location, ok := LocateTemplateError(err)
			if !ok {
				t.Fatalf("LocateTemplateError(%v) did not return a location", err)
			}
			if location.Part != tt.wantPart || location.Line != tt.wantLine {
				t.Fatalf("location = %#v, want part %q line %d (error: %v)", location, tt.wantPart, tt.wantLine, err)
			}
		})
	}
}

func TestTemplateServiceRejectsStaleSave(t *testing.T) {
	fakeStore := newFakeTemplateOverrideStore()
	service := NewTemplateService(fakeStore)
	updaterID := uuid.New()
	input := SaveTemplateOverrideInput{
		Template:         domain.EmailTemplateSignupCode,
		SubjectTemplate:  "Verification",
		TextTemplate:     "Code: {{code}}",
		HTMLTemplate:     "<strong>{{code}}</strong>",
		ExpectedRevision: 0,
		UpdatedByUserID:  updaterID,
	}

	if _, err := service.SaveOverride(context.Background(), input); err != nil {
		t.Fatalf("initial SaveOverride failed: %v", err)
	}
	input.SubjectTemplate = "Stale verification"
	if _, err := service.SaveOverride(context.Background(), input); !errors.Is(err, store.ErrEmailTemplateRevisionConflict) {
		t.Fatalf("stale SaveOverride error = %v", err)
	}
	if got := fakeStore.overrides[input.Template].SubjectTemplate; got != "Verification" {
		t.Fatalf("stale save replaced subject with %q", got)
	}
}

func TestTemplateServiceLoadsHistoricalVersionWithoutMutationAndCanSaveItAsNew(t *testing.T) {
	fakeStore := newFakeTemplateOverrideStore()
	service := NewTemplateService(fakeStore)
	updaterID := uuid.New()
	input := SaveTemplateOverrideInput{
		Template:         domain.EmailTemplateSignupCode,
		SubjectTemplate:  "First subject",
		TextTemplate:     "First code: {{code}}",
		HTMLTemplate:     "<p>First code: {{code}}</p>",
		ExpectedRevision: 0,
		UpdatedByUserID:  updaterID,
	}

	first, err := service.SaveOverride(context.Background(), input)
	if err != nil {
		t.Fatalf("first SaveOverride failed: %v", err)
	}
	input.SubjectTemplate = "Second subject"
	input.TextTemplate = "Second code: {{code}}"
	input.HTMLTemplate = "<p>Second code: {{code}}</p>"
	input.ExpectedRevision = first.Override.Revision
	second, err := service.SaveOverride(context.Background(), input)
	if err != nil {
		t.Fatalf("second SaveOverride failed: %v", err)
	}

	snapshot, err := service.GetVersion(context.Background(), input.Template, 1)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
	if snapshot.SubjectTemplate != "First subject" {
		t.Fatalf("historical snapshot = %#v", snapshot)
	}
	if _, err := service.GetVersion(context.Background(), input.Template, 99); !errors.Is(err, store.ErrEmailTemplateVersionNotFound) {
		t.Fatalf("missing GetVersion error = %v", err)
	}
	versions, err := service.History(context.Background(), input.Template)
	if err != nil {
		t.Fatalf("History failed: %v", err)
	}
	if len(versions) != 2 || fakeStore.overrides[input.Template].SubjectTemplate != "Second subject" {
		t.Fatalf("loading history changed persisted state: versions=%#v override=%#v", versions, fakeStore.overrides[input.Template])
	}

	restored, err := service.SaveOverride(context.Background(), SaveTemplateOverrideInput{
		Template:         input.Template,
		SubjectTemplate:  snapshot.SubjectTemplate,
		TextTemplate:     snapshot.TextTemplate,
		HTMLTemplate:     snapshot.HTMLTemplate,
		ExpectedRevision: second.Override.Revision,
		UpdatedByUserID:  updaterID,
	})
	if err != nil {
		t.Fatalf("SaveOverride from historical snapshot failed: %v", err)
	}
	if restored.Override == nil || restored.Override.Revision != 3 || restored.Override.SubjectTemplate != "First subject" {
		t.Fatalf("restored override = %#v", restored.Override)
	}
	versions, err = service.History(context.Background(), input.Template)
	if err != nil {
		t.Fatalf("History after save failed: %v", err)
	}
	if len(versions) != 3 || versions[0].Version != 3 || versions[0].SubjectTemplate != "First subject" {
		t.Fatalf("versions after save = %#v", versions)
	}

	if err := service.DeleteOverride(context.Background(), input.Template, restored.Override.Revision); err != nil {
		t.Fatalf("DeleteOverride failed: %v", err)
	}
	snapshot, err = service.GetVersion(context.Background(), input.Template, 2)
	if err != nil {
		t.Fatalf("GetVersion after built-in restore failed: %v", err)
	}
	restored, err = service.SaveOverride(context.Background(), SaveTemplateOverrideInput{
		Template:         input.Template,
		SubjectTemplate:  snapshot.SubjectTemplate,
		TextTemplate:     snapshot.TextTemplate,
		HTMLTemplate:     snapshot.HTMLTemplate,
		ExpectedRevision: 0,
		UpdatedByUserID:  updaterID,
	})
	if err != nil {
		t.Fatalf("SaveOverride after built-in restore failed: %v", err)
	}
	if restored.Override == nil || restored.Override.Revision != 1 || restored.Override.SubjectTemplate != "Second subject" {
		t.Fatalf("restored override after built-in restore = %#v", restored.Override)
	}
	versions, err = service.History(context.Background(), input.Template)
	if err != nil {
		t.Fatalf("History after built-in restore failed: %v", err)
	}
	if len(versions) != 4 || versions[0].Version != 4 || versions[0].SubjectTemplate != "Second subject" {
		t.Fatalf("versions after built-in restore = %#v", versions)
	}
}

func TestTemplateServiceListsCatalogWithOverrideState(t *testing.T) {
	fakeStore := newFakeTemplateOverrideStore()
	fakeStore.overrides[domain.EmailTemplatePasswordResetCode] = domain.EmailTemplateOverride{
		Template:        domain.EmailTemplatePasswordResetCode,
		SubjectTemplate: "Reset",
		TextTemplate:    "Reset with {{code}}",
		HTMLTemplate:    "<p>{{code}}</p>",
		Revision:        1,
	}
	service := NewTemplateService(fakeStore)

	templates, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(templates) != len(TemplateCatalog()) {
		t.Fatalf("List returned %d templates, want %d", len(templates), len(TemplateCatalog()))
	}
	for _, template := range templates {
		wantSource := TemplateSourceBuiltIn
		if template.Definition.Key == domain.EmailTemplatePasswordResetCode {
			wantSource = TemplateSourceOverride
		}
		if template.Source != wantSource {
			t.Errorf("template %q source = %q, want %q", template.Definition.Key, template.Source, wantSource)
		}
	}
}

type fakeTemplateOverrideStore struct {
	overrides   map[domain.EmailTemplate]domain.EmailTemplateOverride
	versions    map[domain.EmailTemplate][]domain.EmailTemplateVersion
	upsertCalls int
}

func newFakeTemplateOverrideStore() *fakeTemplateOverrideStore {
	return &fakeTemplateOverrideStore{
		overrides: make(map[domain.EmailTemplate]domain.EmailTemplateOverride),
		versions:  make(map[domain.EmailTemplate][]domain.EmailTemplateVersion),
	}
}

func (s *fakeTemplateOverrideStore) GetEmailTemplateOverride(_ context.Context, key domain.EmailTemplate) (domain.EmailTemplateOverride, error) {
	override, ok := s.overrides[key]
	if !ok {
		return domain.EmailTemplateOverride{}, store.ErrEmailTemplateOverrideNotFound
	}
	return override, nil
}

func (s *fakeTemplateOverrideStore) ListEmailTemplateOverrides(context.Context) ([]domain.EmailTemplateOverride, error) {
	out := make([]domain.EmailTemplateOverride, 0, len(s.overrides))
	for _, override := range s.overrides {
		out = append(out, override)
	}
	return out, nil
}

func (s *fakeTemplateOverrideStore) GetEmailTemplateVersion(_ context.Context, key domain.EmailTemplate, version int64) (domain.EmailTemplateVersion, error) {
	for _, candidate := range s.versions[key] {
		if candidate.Version == version {
			return candidate, nil
		}
	}
	return domain.EmailTemplateVersion{}, store.ErrEmailTemplateVersionNotFound
}

func (s *fakeTemplateOverrideStore) ListEmailTemplateVersions(_ context.Context, key domain.EmailTemplate) ([]domain.EmailTemplateVersion, error) {
	versions := s.versions[key]
	out := make([]domain.EmailTemplateVersion, len(versions))
	for i := range versions {
		out[len(versions)-1-i] = versions[i]
	}
	return out, nil
}

func (s *fakeTemplateOverrideStore) UpsertEmailTemplateOverride(_ context.Context, override domain.EmailTemplateOverride, expectedRevision int64) (domain.EmailTemplateOverride, error) {
	s.upsertCalls++
	current, exists := s.overrides[override.Template]
	if (!exists && expectedRevision != 0) || (exists && current.Revision != expectedRevision) {
		return domain.EmailTemplateOverride{}, store.ErrEmailTemplateRevisionConflict
	}
	override.Revision = current.Revision + 1
	s.overrides[override.Template] = override
	history := s.versions[override.Template]
	s.versions[override.Template] = append(history, domain.EmailTemplateVersion{
		Template:        override.Template,
		Version:         int64(len(history) + 1),
		SubjectTemplate: override.SubjectTemplate,
		TextTemplate:    override.TextTemplate,
		HTMLTemplate:    override.HTMLTemplate,
		CreatedByUserID: override.UpdatedByUserID,
	})
	return override, nil
}

func (s *fakeTemplateOverrideStore) DeleteEmailTemplateOverride(_ context.Context, key domain.EmailTemplate, expectedRevision int64) error {
	current, ok := s.overrides[key]
	if !ok || current.Revision != expectedRevision {
		return store.ErrEmailTemplateRevisionConflict
	}
	delete(s.overrides, key)
	return nil
}
