package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
)

func TestEmailTemplateOverrideStoreLifecycle(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		updater, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "email-template-operator@example.com",
			Username: "email-template-operator",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		input := domain.EmailTemplateOverride{
			Template:        domain.EmailTemplateSignupCode,
			SubjectTemplate: "Custom verification",
			TextTemplate:    "Use {{code}}",
			HTMLTemplate:    "<strong>{{code}}</strong>",
			UpdatedByUserID: &updater.ID,
		}
		created, err := tdb.Store.UpsertEmailTemplateOverride(ctx, input)
		if err != nil {
			t.Fatalf("first UpsertEmailTemplateOverride failed: %v", err)
		}
		if created.Revision != 1 || created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
			t.Fatalf("created override metadata = %#v", created)
		}
		if created.UpdatedByUserID == nil || *created.UpdatedByUserID != updater.ID {
			t.Fatalf("created updated_by = %v, want %s", created.UpdatedByUserID, updater.ID)
		}

		input.SubjectTemplate = "Updated verification"
		updated, err := tdb.Store.UpsertEmailTemplateOverride(ctx, input)
		if err != nil {
			t.Fatalf("second UpsertEmailTemplateOverride failed: %v", err)
		}
		if updated.Revision != 2 || updated.SubjectTemplate != input.SubjectTemplate {
			t.Fatalf("updated override = %#v", updated)
		}
		if !updated.CreatedAt.Equal(created.CreatedAt) {
			t.Fatalf("created_at changed from %s to %s", created.CreatedAt, updated.CreatedAt)
		}

		got, err := tdb.Store.GetEmailTemplateOverride(ctx, input.Template)
		if err != nil {
			t.Fatalf("GetEmailTemplateOverride failed: %v", err)
		}
		if got.Revision != 2 || got.SubjectTemplate != input.SubjectTemplate {
			t.Fatalf("stored override = %#v", got)
		}

		all, err := tdb.Store.ListEmailTemplateOverrides(ctx)
		if err != nil {
			t.Fatalf("ListEmailTemplateOverrides failed: %v", err)
		}
		found := false
		for _, override := range all {
			if override.Template == input.Template {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("ListEmailTemplateOverrides did not include %q", input.Template)
		}

		if err := tdb.Store.DeleteEmailTemplateOverride(ctx, input.Template); err != nil {
			t.Fatalf("DeleteEmailTemplateOverride failed: %v", err)
		}
		if _, err := tdb.Store.GetEmailTemplateOverride(ctx, input.Template); !errors.Is(err, store.ErrEmailTemplateOverrideNotFound) {
			t.Fatalf("GetEmailTemplateOverride after delete error = %v", err)
		}
		if err := tdb.Store.DeleteEmailTemplateOverride(ctx, input.Template); !errors.Is(err, store.ErrEmailTemplateOverrideNotFound) {
			t.Fatalf("second DeleteEmailTemplateOverride error = %v", err)
		}
	})
}
