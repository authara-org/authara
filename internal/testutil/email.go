package testutil

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/store"
)

type emailJobQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func CountEmailJobs(
	t testing.TB,
	ctx context.Context,
	toEmail string,
	template domain.EmailTemplate,
) int {
	t.Helper()
	queryer, ok := ctx.Value(store.DbKey).(emailJobQueryer)
	if !ok {
		t.Fatal("email job assertion requires a transaction context")
	}
	var count int
	if err := queryer.QueryRowContext(ctx, `
		SELECT count(*)
		FROM email_jobs
		WHERE lower(to_email) = lower($1) AND template = $2
	`, toEmail, string(template)).Scan(&count); err != nil {
		t.Fatalf("count email jobs: %v", err)
	}
	return count
}

func CountEmailJobsWithTemplateDataValue(
	t testing.TB,
	ctx context.Context,
	toEmail string,
	template domain.EmailTemplate,
	variable string,
	value string,
) int {
	t.Helper()
	queryer, ok := ctx.Value(store.DbKey).(emailJobQueryer)
	if !ok {
		t.Fatal("email job assertion requires a transaction context")
	}
	var count int
	if err := queryer.QueryRowContext(ctx, `
		SELECT count(*)
		FROM email_jobs
		WHERE lower(to_email) = lower($1)
			AND template = $2
			AND template_data ->> $3 = $4
	`, toEmail, string(template), variable, value).Scan(&count); err != nil {
		t.Fatalf("count email jobs by template data: %v", err)
	}
	return count
}

func LatestEmailTemplateData(
	t testing.TB,
	ctx context.Context,
	toEmail string,
	template domain.EmailTemplate,
) email.TemplateData {
	t.Helper()
	queryer, ok := ctx.Value(store.DbKey).(emailJobQueryer)
	if !ok {
		t.Fatal("email job assertion requires a transaction context")
	}
	var raw []byte
	if err := queryer.QueryRowContext(ctx, `
		SELECT template_data
		FROM email_jobs
		WHERE lower(to_email) = lower($1) AND template = $2
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, toEmail, string(template)).Scan(&raw); err != nil {
		t.Fatalf("load email template data: %v", err)
	}
	var data email.TemplateData
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("decode email template data: %v", err)
	}
	return data
}
