package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
)

var ErrMissingEmailRecipient = errors.New("email recipient is missing")

type JobCreator interface {
	CreateEmailJob(context.Context, domain.EmailJob) (domain.EmailJob, error)
}

// Enqueue creates a transactional email job using the transaction carried by
// ctx, when present. Callers should invoke it in the same transaction as the
// state change described by the email.
func Enqueue(
	ctx context.Context,
	store JobCreator,
	toEmail string,
	template domain.EmailTemplate,
	data TemplateData,
	now time.Time,
) error {
	toEmail = strings.TrimSpace(toEmail)
	if toEmail == "" {
		return ErrMissingEmailRecipient
	}
	definition, err := LookupTemplate(template)
	if err != nil {
		return err
	}
	if err := validateTemplateData(definition, data); err != nil {
		return err
	}
	templateData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal email template data: %w", err)
	}
	_, err = store.CreateEmailJob(ctx, domain.EmailJob{
		ToEmail:       toEmail,
		Template:      template,
		TemplateData:  templateData,
		Status:        domain.EmailJobStatusPending,
		AttemptCount:  0,
		NextAttemptAt: now,
	})
	return err
}

func OccurredAt(now time.Time) string {
	return now.UTC().Format(time.RFC3339)
}

func ValueOrUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Unknown"
	}
	return value
}
