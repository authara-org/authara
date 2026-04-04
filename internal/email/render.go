package email

import (
	"bytes"
	"context"

	emailtemplates "github.com/authara-org/authara/internal/email/templates"
)

func RenderSignupCodeHTML(code string) (string, error) {
	var buf bytes.Buffer
	err := emailtemplates.SignupCodeEmail(code).Render(context.Background(), &buf)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
