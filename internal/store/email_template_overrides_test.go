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
		created, err := tdb.Store.UpsertEmailTemplateOverride(ctx, input, 0)
		if err != nil {
			t.Fatalf("first UpsertEmailTemplateOverride failed: %v", err)
		}
		if created.Revision != 1 || created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
			t.Fatalf("created override metadata = %#v", created)
		}
		if created.UpdatedByUserID == nil || *created.UpdatedByUserID != updater.ID {
			t.Fatalf("created updated_by = %v, want %s", created.UpdatedByUserID, updater.ID)
		}
		versions, err := tdb.Store.ListEmailTemplateVersions(ctx, input.Template)
		if err != nil {
			t.Fatalf("ListEmailTemplateVersions after create failed: %v", err)
		}
		if len(versions) != 1 || versions[0].Version != 1 || versions[0].SubjectTemplate != input.SubjectTemplate {
			t.Fatalf("versions after create = %#v", versions)
		}

		input.SubjectTemplate = "Updated verification"
		updated, err := tdb.Store.UpsertEmailTemplateOverride(ctx, input, created.Revision)
		if err != nil {
			t.Fatalf("second UpsertEmailTemplateOverride failed: %v", err)
		}
		if updated.Revision != 2 || updated.SubjectTemplate != input.SubjectTemplate {
			t.Fatalf("updated override = %#v", updated)
		}
		if !updated.CreatedAt.Equal(created.CreatedAt) {
			t.Fatalf("created_at changed from %s to %s", created.CreatedAt, updated.CreatedAt)
		}
		versions, err = tdb.Store.ListEmailTemplateVersions(ctx, input.Template)
		if err != nil {
			t.Fatalf("ListEmailTemplateVersions after update failed: %v", err)
		}
		if len(versions) != 2 || versions[0].Version != 2 || versions[0].SubjectTemplate != input.SubjectTemplate {
			t.Fatalf("versions after update = %#v", versions)
		}
		firstVersion, err := tdb.Store.GetEmailTemplateVersion(ctx, input.Template, 1)
		if err != nil {
			t.Fatalf("GetEmailTemplateVersion failed: %v", err)
		}
		if firstVersion.SubjectTemplate != "Custom verification" {
			t.Fatalf("first version subject = %q", firstVersion.SubjectTemplate)
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

		input.SubjectTemplate = "Stale update"
		if _, err := tdb.Store.UpsertEmailTemplateOverride(ctx, input, created.Revision); !errors.Is(err, store.ErrEmailTemplateRevisionConflict) {
			t.Fatalf("stale UpsertEmailTemplateOverride error = %v", err)
		}

		if err := tdb.Store.DeleteEmailTemplateOverride(ctx, input.Template, created.Revision); !errors.Is(err, store.ErrEmailTemplateRevisionConflict) {
			t.Fatalf("stale DeleteEmailTemplateOverride error = %v", err)
		}
		if err := tdb.Store.DeleteEmailTemplateOverride(ctx, input.Template, updated.Revision); err != nil {
			t.Fatalf("DeleteEmailTemplateOverride failed: %v", err)
		}
		if _, err := tdb.Store.GetEmailTemplateOverride(ctx, input.Template); !errors.Is(err, store.ErrEmailTemplateOverrideNotFound) {
			t.Fatalf("GetEmailTemplateOverride after delete error = %v", err)
		}
		versions, err = tdb.Store.ListEmailTemplateVersions(ctx, input.Template)
		if err != nil || len(versions) != 2 {
			t.Fatalf("history after override delete = %#v, err=%v", versions, err)
		}
		if _, err := tdb.Store.GetEmailTemplateVersion(ctx, input.Template, 99); !errors.Is(err, store.ErrEmailTemplateVersionNotFound) {
			t.Fatalf("missing GetEmailTemplateVersion error = %v", err)
		}
		if err := tdb.Store.DeleteEmailTemplateOverride(ctx, input.Template, updated.Revision); !errors.Is(err, store.ErrEmailTemplateRevisionConflict) {
			t.Fatalf("second DeleteEmailTemplateOverride error = %v", err)
		}
	})
}
