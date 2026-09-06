package email

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/domain"
)

func TestTemplateCatalogCoversSupportedDomainTemplates(t *testing.T) {
	catalog := TemplateCatalog()
	supported := domain.SupportedEmailTemplates()

	if len(catalog) != len(supported) {
		t.Fatalf("catalog contains %d definitions, supported templates contains %d", len(catalog), len(supported))
	}

	seen := make(map[domain.EmailTemplate]struct{}, len(catalog))
	for i, definition := range catalog {
		if definition.Key != supported[i] {
			t.Errorf("catalog entry %d is %q, want %q", i, definition.Key, supported[i])
		}
		if _, duplicate := seen[definition.Key]; duplicate {
			t.Fatalf("catalog contains duplicate definition for %q", definition.Key)
		}
		seen[definition.Key] = struct{}{}
	}

	for _, key := range supported {
		if _, ok := seen[key]; !ok {
			t.Errorf("catalog is missing supported template %q", key)
		}
	}
}

func TestTemplateCatalogDefinitionsAreCompleteAndRenderable(t *testing.T) {
	for _, definition := range TemplateCatalog() {
		t.Run(string(definition.Key), func(t *testing.T) {
			if definition.DisplayName == "" {
				t.Error("display name is empty")
			}
			if definition.Description == "" {
				t.Error("description is empty")
			}
			if definition.DefaultSubjectTemplate == "" {
				t.Error("subject template is empty")
			}
			if definition.DefaultTextTemplate == "" {
				t.Error("text template is empty")
			}
			if definition.DefaultHTMLTemplate == "" {
				t.Error("HTML template is empty")
			}

			seenVariables := make(map[string]struct{})
			for _, variable := range definition.AvailableVariables {
				if variable == "" {
					t.Error("template contains an empty variable name")
				}
				if _, duplicate := seenVariables[variable]; duplicate {
					t.Errorf("template contains duplicate variable %q", variable)
				}
				seenVariables[variable] = struct{}{}
			}

			for _, variable := range definition.AvailableVariables {
				if strings.TrimSpace(definition.SampleData[variable]) == "" {
					t.Errorf("sample data is missing available variable %q", variable)
				}
			}

			message, err := RenderBuiltInTemplate(definition.Key, definition.SampleData)
			if err != nil {
				t.Fatalf("render built-in template: %v", err)
			}
			if message.Subject == "" || message.Text == "" || message.HTML == "" {
				t.Fatalf("rendered message contains an empty part: %+v", message)
			}
		})
	}
}

func TestTemplateCatalogRejectsUnknownTemplate(t *testing.T) {
	unknown := domain.EmailTemplate("unknown")

	if err := ValidateTemplate(unknown); !errors.Is(err, ErrUnknownTemplate) {
		t.Fatalf("ValidateTemplate error = %v, want ErrUnknownTemplate", err)
	}
	if _, err := RenderBuiltInTemplate(unknown, nil); !errors.Is(err, ErrUnknownTemplate) {
		t.Fatalf("RenderBuiltInTemplate error = %v, want ErrUnknownTemplate", err)
	}
}

func TestTemplateDefinitionRequiresDeclaredVariables(t *testing.T) {
	_, err := RenderBuiltInTemplate(domain.EmailTemplateSignupCode, TemplateData{})
	if !errors.Is(err, ErrMissingTemplateVariable) {
		t.Fatalf("RenderBuiltInTemplate error = %v, want ErrMissingTemplateVariable", err)
	}
}

func TestTemplateCatalogReturnsIndependentMetadata(t *testing.T) {
	first := TemplateCatalog()
	first[0].AvailableVariables[0] = "changed"
	first[0].RequiredBodyVariables[0] = "changed"
	first[0].SampleData[TemplateVariableCode] = "changed"

	second := TemplateCatalog()
	if reflect.DeepEqual(first[0].AvailableVariables, second[0].AvailableVariables) {
		t.Fatal("available variables share mutable catalog state")
	}
	if reflect.DeepEqual(first[0].RequiredBodyVariables, second[0].RequiredBodyVariables) {
		t.Fatal("required body variables share mutable catalog state")
	}
	if reflect.DeepEqual(first[0].SampleData, second[0].SampleData) {
		t.Fatal("sample data shares mutable catalog state")
	}
}
