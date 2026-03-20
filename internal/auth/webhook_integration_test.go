package auth

import (
	"context"
	"testing"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/authara-org/authara/internal/webhook"
)

type recordingPublisher struct {
	events []webhook.Envelope
}

func (p *recordingPublisher) Publish(ctx context.Context, evt webhook.Envelope) error {
	p.events = append(p.events, evt)
	return nil
}

func TestSignup_EmitsUserCreated(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		pub := &recordingPublisher{}

		svc := New(Config{
			Store:            tdb.Store,
			Tx:               tdb.Tx,
			WebhookPublisher: pub,
		})

		user, err := svc.Signup(ctx, SignupInput{
			Email:    "webhook-signup@example.com",
			Username: "webhook-signup",
			Password: "very-secure-password",
			Provider: domain.ProviderPassword,
		})
		if err != nil {
			t.Fatalf("Signup failed: %v", err)
		}

		if len(pub.events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(pub.events))
		}

		evt := pub.events[0]
		if evt.Type != webhook.EventUserCreated {
			t.Fatalf("expected event %q, got %q", webhook.EventUserCreated, evt.Type)
		}

		data, ok := evt.Data.(webhook.UserData)
		if !ok {
			t.Fatalf("expected event data type webhook.UserData, got %T", evt.Data)
		}

		if data.UserID != user.ID {
			t.Fatalf("expected user_id %q, got %q", user.ID, data.UserID)
		}
		if evt.ID == "" {
			t.Fatal("expected non-empty event id")
		}
		if evt.CreatedAt.IsZero() {
			t.Fatal("expected non-zero occurred_at")
		}
	})
}

func TestDeleteUser_EmitsUserDeleted(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		pub := &recordingPublisher{}

		svc := New(Config{
			Store:            tdb.Store,
			Tx:               tdb.Tx,
			WebhookPublisher: pub,
		})

		createdUser, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "webhook-delete@example.com",
			Username: "webhook-delete",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		if err := svc.DeleteUser(ctx, createdUser.ID); err != nil {
			t.Fatalf("DeleteUser failed: %v", err)
		}

		if len(pub.events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(pub.events))
		}

		evt := pub.events[0]
		if evt.Type != webhook.EventUserDeleted {
			t.Fatalf("expected event %q, got %q", webhook.EventUserDeleted, evt.Type)
		}

		data, ok := evt.Data.(webhook.UserData)
		if !ok {
			t.Fatalf("expected event data type webhook.UserData, got %T", evt.Data)
		}

		if data.UserID != createdUser.ID {
			t.Fatalf("expected user_id %q, got %q", createdUser.ID, data.UserID)
		}
	})
}
