package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
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
		auditFilter := store.OperatorAuditEventFilter{
			ActorUserID:  &updater.ID,
			ResourceType: domain.OperatorAuditResourceEmailTemplate,
			ResourceID:   string(input.Template),
		}
		events, err := tdb.Store.ListOperatorAuditEvents(ctx, auditFilter)
		if err != nil {
			t.Fatalf("ListOperatorAuditEvents after saves failed: %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("audit events after saves = %#v", events)
		}
		for _, event := range events {
			if event.Action != domain.OperatorAuditActionEmailTemplateSaved {
				t.Fatalf("save audit action = %q", event.Action)
			}
			var metadata map[string]any
			if err := json.Unmarshal(event.Metadata, &metadata); err != nil {
				t.Fatalf("decode audit metadata: %v", err)
			}
			if _, ok := metadata["revision"]; !ok {
				t.Fatalf("save audit metadata lacks revision: %s", event.Metadata)
			}
			if _, ok := metadata["version"]; !ok {
				t.Fatalf("save audit metadata lacks version: %s", event.Metadata)
			}
			for _, sensitiveKey := range []string{"subject_template", "text_template", "html_template"} {
				if _, ok := metadata[sensitiveKey]; ok {
					t.Fatalf("audit metadata contains template source %q: %s", sensitiveKey, event.Metadata)
				}
			}
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

		if err := tdb.Store.DeleteEmailTemplateOverride(ctx, input.Template, created.Revision, updater.ID); !errors.Is(err, store.ErrEmailTemplateRevisionConflict) {
			t.Fatalf("stale DeleteEmailTemplateOverride error = %v", err)
		}
		if err := tdb.Store.DeleteEmailTemplateOverride(ctx, input.Template, updated.Revision, updater.ID); err != nil {
			t.Fatalf("DeleteEmailTemplateOverride failed: %v", err)
		}
		events, err = tdb.Store.ListOperatorAuditEvents(ctx, auditFilter)
		if err != nil {
			t.Fatalf("ListOperatorAuditEvents after restore failed: %v", err)
		}
		if len(events) != 3 {
			t.Fatalf("audit events after restore = %#v", events)
		}
		restoreEvents, err := tdb.Store.ListOperatorAuditEvents(ctx, store.OperatorAuditEventFilter{
			ActorUserID:  &updater.ID,
			Action:       domain.OperatorAuditActionEmailTemplateRestoredBuiltIn,
			ResourceType: domain.OperatorAuditResourceEmailTemplate,
			ResourceID:   string(input.Template),
		})
		if err != nil || len(restoreEvents) != 1 {
			t.Fatalf("restore audit events = %#v, err=%v", restoreEvents, err)
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
		if err := tdb.Store.DeleteEmailTemplateOverride(ctx, input.Template, updated.Revision, updater.ID); !errors.Is(err, store.ErrEmailTemplateRevisionConflict) {
			t.Fatalf("second DeleteEmailTemplateOverride error = %v", err)
		}
	})
}

func TestEmailTemplateDeliverySettingControlsJobCreation(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		operator, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "email-delivery-operator@example.com",
			Username: "email-delivery-operator",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		now := time.Now().UTC()

		defaultJob, err := tdb.Store.CreateEmailJob(ctx, domain.EmailJob{
			ToEmail:       "default-enabled@example.com",
			Template:      domain.EmailTemplatePasswordResetCode,
			Status:        domain.EmailJobStatusPending,
			NextAttemptAt: now,
		})
		if err != nil || defaultJob.ID == uuid.Nil {
			t.Fatalf("default-enabled CreateEmailJob = %#v, err=%v", defaultJob, err)
		}

		disabled, err := tdb.Store.SetEmailTemplateDeliveryEnabled(
			ctx,
			domain.EmailTemplateNewSignIn,
			false,
			operator.ID,
		)
		if err != nil {
			t.Fatalf("disable delivery failed: %v", err)
		}
		if disabled.Enabled || disabled.Template != domain.EmailTemplateNewSignIn ||
			disabled.UpdatedByUserID == nil || *disabled.UpdatedByUserID != operator.ID {
			t.Fatalf("disabled setting = %#v", disabled)
		}

		suppressedJob, err := tdb.Store.CreateEmailJob(ctx, domain.EmailJob{
			ToEmail:       "suppressed@example.com",
			Template:      domain.EmailTemplateNewSignIn,
			Status:        domain.EmailJobStatusPending,
			NextAttemptAt: now,
		})
		if err != nil {
			t.Fatalf("disabled CreateEmailJob failed: %v", err)
		}
		if suppressedJob.ID != uuid.Nil {
			t.Fatalf("disabled CreateEmailJob returned job %#v", suppressedJob)
		}
		if got := testutil.CountEmailJobs(t, ctx, "suppressed@example.com", domain.EmailTemplateNewSignIn); got != 0 {
			t.Fatalf("disabled email jobs = %d, want 0", got)
		}

		enabled, err := tdb.Store.SetEmailTemplateDeliveryEnabled(
			ctx,
			domain.EmailTemplateNewSignIn,
			true,
			operator.ID,
		)
		if err != nil || !enabled.Enabled {
			t.Fatalf("enable delivery = %#v, err=%v", enabled, err)
		}
		createdJob, err := tdb.Store.CreateEmailJob(ctx, domain.EmailJob{
			ToEmail:       "enabled@example.com",
			Template:      domain.EmailTemplateNewSignIn,
			Status:        domain.EmailJobStatusPending,
			NextAttemptAt: now,
		})
		if err != nil || createdJob.ID == uuid.Nil {
			t.Fatalf("enabled CreateEmailJob = %#v, err=%v", createdJob, err)
		}

		settings, err := tdb.Store.ListEmailTemplateDeliverySettings(ctx)
		if err != nil {
			t.Fatalf("ListEmailTemplateDeliverySettings failed: %v", err)
		}
		if len(settings) != 1 || settings[0].Template != domain.EmailTemplateNewSignIn || !settings[0].Enabled {
			t.Fatalf("delivery settings = %#v", settings)
		}

		events, err := tdb.Store.ListOperatorAuditEvents(ctx, store.OperatorAuditEventFilter{
			ActorUserID:  &operator.ID,
			ResourceType: domain.OperatorAuditResourceEmailTemplate,
			ResourceID:   string(domain.EmailTemplateNewSignIn),
		})
		if err != nil {
			t.Fatalf("ListOperatorAuditEvents failed: %v", err)
		}
		if len(events) != 2 ||
			events[0].Action != domain.OperatorAuditActionEmailTemplateDeliveryEnabled ||
			events[1].Action != domain.OperatorAuditActionEmailTemplateDeliveryDisabled {
			t.Fatalf("delivery audit events = %#v", events)
		}
	})
}
