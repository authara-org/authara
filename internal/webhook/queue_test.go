package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

func TestQueuePublisherUsesCurrentTransaction(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	publisher := NewQueuePublisher(tdb.Store)
	event := NewUserCreated(uuid.New(), time.Now().UTC())

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		if err := publisher.Publish(ctx, event); err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
		stored, err := tdb.Store.GetWebhookEventByID(ctx, event.ID)
		if err != nil {
			t.Fatalf("GetWebhookEventByID failed: %v", err)
		}
		if stored.EventType != string(event.Type) {
			t.Fatalf("expected event type %q, got %q", event.Type, stored.EventType)
		}
	})

	_, err := tdb.Store.GetWebhookEventByID(context.Background(), event.ID)
	if !errors.Is(err, store.ErrorWebhookEventNotFound) {
		t.Fatalf("expected rolled-back event to be absent, got %v", err)
	}
}

func TestWorkerDeliversQueuedEvent(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	event := NewUserCreated(uuid.New(), time.Now().UTC())
	defer deleteWebhookEvent(t, tdb, event.ID)

	if err := NewQueuePublisher(tdb.Store).Publish(context.Background(), event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	var received Envelope
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(EventHeader); got != string(event.Type) {
			t.Errorf("expected event header %q, got %q", event.Type, got)
		}
		if got := r.Header.Get(DeliveryHeader); got != event.ID {
			t.Errorf("expected delivery header %q, got %q", event.ID, got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode webhook: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	worker := NewWorker(
		tdb.Store,
		NewSender(server.URL, "secret", server.Client()),
		nil,
		WorkerConfig{WorkerCount: 1, PollInterval: time.Second},
	)
	processed, err := worker.RunOnce(context.Background(), time.Now().UTC())
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	if !processed {
		t.Fatal("expected a webhook event to be processed")
	}
	if received.ID != event.ID || received.Type != event.Type {
		t.Fatalf("unexpected delivered event: %+v", received)
	}

	stored, err := tdb.Store.GetWebhookEventByID(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("GetWebhookEventByID failed: %v", err)
	}
	if stored.Status != domain.WebhookEventStatusDelivered || stored.DeliveredAt == nil {
		t.Fatalf("expected delivered event, got %+v", stored)
	}
	if stored.AttemptCount != 1 {
		t.Fatalf("expected one attempt, got %d", stored.AttemptCount)
	}
}

func deleteWebhookEvent(t *testing.T, tdb *testutil.TestDB, eventID string) {
	t.Helper()
	if _, err := tdb.Store.DB().Exec(`DELETE FROM webhook_events WHERE id = $1`, eventID); err != nil {
		t.Errorf("delete webhook event: %v", err)
	}
}
