package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
)

func TestRuntimeSettingPersistenceAndAudit(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		actor, err := tdb.Store.CreateUser(ctx, domain.User{Email: "runtime-settings@example.com", Username: "runtime-settings"})
		if err != nil {
			t.Fatal(err)
		}
		service, err := config.NewService(ctx, config.ServiceOptions{Startup: &config.Config{},
			Store:             tdb.Store,
			LookupEnvironment: func(string) (string, bool) { return "", false },
		})
		if err != nil {
			t.Fatal(err)
		}

		description, err := service.Set(ctx, config.KeyChallengeMaxAttempts, "8", actor.ID, 0)
		if err != nil {
			t.Fatalf("Set: %v", err)
		}
		if description.Revision <= 0 || service.Current().MaxAttempts != 8 {
			t.Fatalf("description = %+v policy = %+v", description, service.Current())
		}
		if _, err := service.Set(ctx, config.KeyChallengeMaxAttempts, "9", actor.ID, 0); !errors.Is(err, config.ErrRevisionConflict) {
			t.Fatalf("stale Set error = %v", err)
		}

		persisted, err := config.NewService(ctx, config.ServiceOptions{Startup: &config.Config{},
			Store:             tdb.Store,
			LookupEnvironment: func(string) (string, bool) { return "", false },
		})
		if err != nil {
			t.Fatal(err)
		}
		if persisted.Current().MaxAttempts != 8 {
			t.Fatalf("reconstructed policy = %+v", persisted.Current())
		}

		if _, err := service.Clear(ctx, config.KeyChallengeMaxAttempts, actor.ID, description.Revision); err != nil {
			t.Fatalf("Clear: %v", err)
		}
		state, err := tdb.Store.LoadRuntimeSettings(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(state.Overrides) != 0 || state.Revision <= description.Revision {
			t.Fatalf("persisted state after clear = %+v", state)
		}

		events, err := tdb.Store.ListOperatorAuditEvents(ctx, store.OperatorAuditEventFilter{
			ResourceType: domain.OperatorAuditResourceRuntimeSetting,
			ResourceID:   string(config.KeyChallengeMaxAttempts),
		})
		if err != nil {
			t.Fatal(err)
		}
		actions := make(map[string]bool, len(events))
		for _, event := range events {
			actions[event.Action] = true
		}
		if len(events) != 2 || !actions[domain.OperatorAuditActionRuntimeSettingCleared] || !actions[domain.OperatorAuditActionRuntimeSettingSet] {
			t.Fatalf("audit events = %+v", events)
		}
	})
}

func TestRuntimeSettingPersistenceRejectsAStaleGlobalRevision(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		actor, err := tdb.Store.CreateUser(ctx, domain.User{Email: "runtime-settings-global-revision@example.com", Username: "runtime-settings-global-revision"})
		if err != nil {
			t.Fatal(err)
		}
		state, err := tdb.Store.LoadRuntimeSettings(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tdb.Store.UpsertRuntimeSettingOverride(ctx, config.Mutation{
			Key: config.KeyChallengeTTL, Value: json.RawMessage(`"1h"`), ExpectedRevision: 0,
			ExpectedStateRevision: state.Revision, ActorUserID: actor.ID, AuditMetadata: json.RawMessage(`{}`),
		}); err != nil {
			t.Fatalf("first mutation: %v", err)
		}
		if _, err := tdb.Store.UpsertRuntimeSettingOverride(ctx, config.Mutation{
			Key: config.KeyChallengeMaxAttempts, Value: json.RawMessage(`8`), ExpectedRevision: 0,
			ExpectedStateRevision: state.Revision, ActorUserID: actor.ID, AuditMetadata: json.RawMessage(`{}`),
		}); !errors.Is(err, config.ErrRevisionConflict) {
			t.Fatalf("stale global revision error = %v", err)
		}
	})
}
