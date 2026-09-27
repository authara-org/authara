package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

func TestSecurityEventFailureRollsBackOwningMutation(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	email := "security-event-rollback-" + uuid.NewString() + "@example.com"
	err := tdb.Tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if _, err := tdb.Store.CreateUser(txCtx, domain.User{Email: email, Username: "security-event-rollback-" + uuid.NewString()}); err != nil {
			return err
		}
		_, err := tdb.Store.CreateSecurityEvent(txCtx, domain.SecurityEvent{
			Type:      domain.SecurityEventCredentialPasswordAdded,
			Outcome:   domain.SecurityEventOutcomeSuccess,
			ActorType: domain.SecurityEventActorType("invalid"),
		})
		return err
	})
	if err == nil {
		t.Fatal("invalid event unexpectedly committed")
	}
	if _, err := tdb.Store.GetUserByEmail(ctx, email); err != store.ErrUserNotFound {
		t.Fatalf("owning mutation survived event failure: %v", err)
	}
}

func TestSecurityEventRoundTripAndFilters(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		userID := uuid.New()
		sessionID := uuid.New()
		organizationID := uuid.New()
		created, err := tdb.Store.CreateSecurityEvent(ctx, domain.SecurityEvent{
			Type:                 domain.SecurityEventAuthenticationLogin,
			Outcome:              domain.SecurityEventOutcomeDenied,
			ReasonCode:           domain.SecurityEventReasonInvalidCredentials,
			ActorType:            domain.SecurityEventActorAnonymous,
			UserID:               &userID,
			SessionID:            &sessionID,
			OrganizationID:       &organizationID,
			AuthenticationMethod: domain.AuthenticationMethodPassword,
		})
		if err != nil {
			t.Fatal(err)
		}
		if created.ID == uuid.Nil || created.CreatedAt.IsZero() {
			t.Fatalf("event did not receive durable identity and timestamp: %+v", created)
		}

		events, err := tdb.Store.QuerySecurityEvents(ctx, store.SecurityEventFilter{
			Type:      domain.SecurityEventAuthenticationLogin,
			Outcome:   domain.SecurityEventOutcomeDenied,
			UserID:    &userID,
			SessionID: &sessionID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 {
			t.Fatalf("filtered events = %d, want 1", len(events))
		}
		got := events[0]
		if got.ReasonCode != domain.SecurityEventReasonInvalidCredentials ||
			got.AuthenticationMethod != domain.AuthenticationMethodPassword ||
			got.ActorType != domain.SecurityEventActorAnonymous ||
			got.OrganizationID == nil || *got.OrganizationID != organizationID {
			t.Fatalf("unexpected security event: %+v", got)
		}
	})
}

func TestDeleteSecurityEventsBeforeUsesRetentionCutoff(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		if _, err := tdb.Store.CreateSecurityEvent(ctx, domain.SecurityEvent{
			Type:      domain.SecurityEventSessionLogout,
			Outcome:   domain.SecurityEventOutcomeSuccess,
			ActorType: domain.SecurityEventActorUser,
		}); err != nil {
			t.Fatal(err)
		}

		deleted, err := tdb.Store.DeleteSecurityEventsBefore(ctx, time.Now().UTC().Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if deleted != 1 {
			t.Fatalf("deleted = %d, want 1", deleted)
		}
		events, err := tdb.Store.ListSecurityEvents(ctx, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 0 {
			t.Fatalf("retained expired events: %+v", events)
		}
	})
}

func TestSecurityEventIdentifiersSurviveSubjectDeletion(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "security-event-delete-" + uuid.NewString() + "@example.com",
			Username: "security-event-delete-" + uuid.NewString(),
		})
		if err != nil {
			t.Fatal(err)
		}
		event := domain.SecurityEvent{
			Type: domain.SecurityEventCredentialPasswordChanged, Outcome: domain.SecurityEventOutcomeSuccess,
			ActorType: domain.SecurityEventActorUser, ActorUserID: &user.ID, UserID: &user.ID,
		}
		created, err := tdb.Store.CreateSecurityEvent(ctx, event)
		if err != nil {
			t.Fatal(err)
		}
		if err := tdb.Store.DeleteUser(ctx, user.ID); err != nil {
			t.Fatal(err)
		}
		events, err := tdb.Store.QuerySecurityEvents(ctx, store.SecurityEventFilter{UserID: &user.ID})
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 || events[0].ID != created.ID || events[0].UserID == nil || *events[0].UserID != user.ID {
			t.Fatalf("security event did not preserve stable subject id: %+v", events)
		}
	})
}
