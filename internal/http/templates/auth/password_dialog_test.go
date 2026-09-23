package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func TestPasswordDialogsRenderModalSubmissionForms(t *testing.T) {
	for _, test := range []struct {
		name      string
		component templ.Component
		heading   string
	}{
		{name: "add", component: AddPasswordDialog(), heading: "Add password"},
		{name: "change", component: ChangePasswordDialog(), heading: "Change password"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var body strings.Builder
			if err := test.component.Render(context.Background(), &body); err != nil {
				t.Fatal(err)
			}
			html := body.String()
			for _, want := range []string{test.heading, `name="modal" value="1"`, "autofocus"} {
				if !strings.Contains(html, want) {
					t.Errorf("password dialog does not contain %q", want)
				}
			}
			if strings.Contains(html, "min-h-screen") {
				t.Error("password dialog must not render the full-page auth layout")
			}
		})
	}
}
