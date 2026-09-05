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
		Template:        domain.EmailTemplateSignupCode,
		SubjectTemplate: "Custom verification",
		TextTemplate:    "Code: {{code}}",
		HTMLTemplate:    "<strong>{{code}}</strong>",
		UpdatedByUserID: updaterID,
	}

	first, err := service.SaveOverride(context.Background(), input)
	if err != nil {
		t.Fatalf("first SaveOverride failed: %v", err)
	}
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
	upsertCalls int
}

func newFakeTemplateOverrideStore() *fakeTemplateOverrideStore {
	return &fakeTemplateOverrideStore{overrides: make(map[domain.EmailTemplate]domain.EmailTemplateOverride)}
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

func (s *fakeTemplateOverrideStore) UpsertEmailTemplateOverride(_ context.Context, override domain.EmailTemplateOverride) (domain.EmailTemplateOverride, error) {
	s.upsertCalls++
	override.Revision = s.overrides[override.Template].Revision + 1
	s.overrides[override.Template] = override
	return override, nil
}

func (s *fakeTemplateOverrideStore) DeleteEmailTemplateOverride(_ context.Context, key domain.EmailTemplate) error {
	if _, ok := s.overrides[key]; !ok {
		return store.ErrEmailTemplateOverrideNotFound
	}
	delete(s.overrides, key)
	return nil
}
