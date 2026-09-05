package operator

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/email"
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
	html := renderOperatorComponent(t, EmailTemplates(definitions))

	for _, definition := range definitions {
		for _, want := range []string{definition.DisplayName, definition.Description, string(definition.Key)} {
			if !strings.Contains(html, want) {
				t.Fatalf("expected email catalog to contain %q", want)
			}
		}
		for _, variable := range append(definition.RequiredVariables, definition.OptionalVariables...) {
			if !strings.Contains(html, "{{"+variable+"}}") {
				t.Fatalf("expected email catalog to contain variable %q", variable)
			}
		}
	}
	if strings.Contains(html, "123456") {
		t.Fatal("email catalog must not render sample template values")
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
