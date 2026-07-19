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

	worker := newTestWorker(tdb, server)
	runWorkerOnce(t, worker, time.Now().UTC())
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

func TestWorkerRetriesTransientFailuresAsQueueAttempts(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	event := NewUserCreated(uuid.New(), time.Now().UTC())
	defer deleteWebhookEvent(t, tdb, event.ID)

	if err := NewQueuePublisher(tdb.Store).Publish(context.Background(), event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	worker := newTestWorker(tdb, server)
	firstAttempt := time.Now().UTC().Add(time.Second)

	runWorkerOnce(t, worker, firstAttempt)
	if calls != 1 {
		t.Fatalf("expected one HTTP call in the first queue attempt, got %d", calls)
	}

	stored := getWebhookEvent(t, tdb, event.ID)
	if stored.Status != domain.WebhookEventStatusPending || stored.AttemptCount != 1 {
		t.Fatalf("expected first retry to be pending, got %+v", stored)
	}
	assertTimeNear(t, stored.NextAttemptAt, firstAttempt.Add(30*time.Second))

	processed, err := worker.RunOnce(context.Background(), firstAttempt.Add(29*time.Second))
	if err != nil || processed {
		t.Fatalf("early RunOnce = (%v, %v), want (false, nil)", processed, err)
	}
	if calls != 1 {
		t.Fatalf("expected no early HTTP retry, got %d calls", calls)
	}

	secondAttempt := firstAttempt.Add(30 * time.Second)
	runWorkerOnce(t, worker, secondAttempt)
	stored = getWebhookEvent(t, tdb, event.ID)
	if stored.Status != domain.WebhookEventStatusPending || stored.AttemptCount != 2 {
		t.Fatalf("expected second retry to be pending, got %+v", stored)
	}
	assertTimeNear(t, stored.NextAttemptAt, secondAttempt.Add(2*time.Minute))

	thirdAttempt := secondAttempt.Add(2 * time.Minute)
	runWorkerOnce(t, worker, thirdAttempt)
	stored = getWebhookEvent(t, tdb, event.ID)
	if stored.Status != domain.WebhookEventStatusFailed || stored.AttemptCount != worker.cfg.MaxDeliveryAttempts {
		t.Fatalf("expected terminal failure on third attempt, got %+v", stored)
	}
	if calls != worker.cfg.MaxDeliveryAttempts {
		t.Fatalf("expected %d total HTTP calls, got %d", worker.cfg.MaxDeliveryAttempts, calls)
	}
}

func TestWorkerReapsStaleProcessingEvents(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	event := NewUserCreated(uuid.New(), time.Now().UTC())
	defer deleteWebhookEvent(t, tdb, event.ID)

	if err := NewQueuePublisher(tdb.Store).Publish(context.Background(), event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	worker := NewWorker(tdb.Store, nil, nil, testWorkerConfig())
	claimAt := time.Now().UTC().Add(time.Second)
	claimed, err := tdb.Store.ClaimNextWebhookEvent(context.Background(), claimAt)
	if err != nil {
		t.Fatalf("ClaimNextWebhookEvent failed: %v", err)
	}

	reaped, err := worker.reapStale(context.Background(), claimAt.Add(worker.cfg.ProcessingStaleAfter-time.Millisecond))
	if err != nil || reaped != 0 {
		t.Fatalf("early ReapStale = (%d, %v), want (0, nil)", reaped, err)
	}

	reaped, err = worker.reapStale(context.Background(), claimAt.Add(worker.cfg.ProcessingStaleAfter))
	if err != nil || reaped != 1 {
		t.Fatalf("ReapStale = (%d, %v), want (1, nil)", reaped, err)
	}
	stored := getWebhookEvent(t, tdb, event.ID)
	if stored.Status != domain.WebhookEventStatusPending || stored.ProcessingStartedAt != nil {
		t.Fatalf("expected stale event to be requeued, got %+v", stored)
	}

	err = tdb.Store.MarkWebhookEventDelivered(
		context.Background(),
		event.ID,
		*claimed.ProcessingStartedAt,
		claimAt.Add(worker.cfg.ProcessingStaleAfter),
	)
	if !errors.Is(err, store.ErrorWebhookEventLeaseLost) {
		t.Fatalf("expected old processing lease to be rejected, got %v", err)
	}
}

func TestWorkerCleanupAppliesTerminalEventRetention(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	deliveredEvent := NewUserCreated(uuid.New(), time.Now().UTC())
	failedEvent := NewUserCreated(uuid.New(), time.Now().UTC())
	defer deleteWebhookEvent(t, tdb, deliveredEvent.ID)
	defer deleteWebhookEvent(t, tdb, failedEvent.ID)

	publisher := NewQueuePublisher(tdb.Store)
	if err := publisher.Publish(context.Background(), deliveredEvent); err != nil {
		t.Fatalf("publish delivered event: %v", err)
	}
	if err := publisher.Publish(context.Background(), failedEvent); err != nil {
		t.Fatalf("publish failed event: %v", err)
	}
	deliveredAt := time.Now().UTC()
	if _, err := tdb.Store.DB().Exec(`
		UPDATE webhook_events
		SET status = CASE WHEN id = $1 THEN 'delivered' ELSE 'failed' END,
		    delivered_at = CASE WHEN id = $1 THEN $3::timestamptz ELSE NULL END,
		    last_error = CASE WHEN id = $2 THEN 'test failure' ELSE NULL END
		WHERE id IN ($1, $2)
	`, deliveredEvent.ID, failedEvent.ID, deliveredAt); err != nil {
		t.Fatalf("seed terminal events: %v", err)
	}

	worker := NewWorker(tdb.Store, nil, nil, testWorkerConfig())
	updatedAt := getWebhookEvent(t, tdb, deliveredEvent.ID).UpdatedAt
	deleted, err := worker.cleanup(context.Background(), updatedAt.Add(worker.cfg.DeliveredRetention-time.Second))
	if err != nil || deleted != 0 {
		t.Fatalf("early cleanup = (%d, %v), want (0, nil)", deleted, err)
	}

	deleted, err = worker.cleanup(context.Background(), updatedAt.Add(worker.cfg.FailedRetention+time.Second))
	if err != nil || deleted != 2 {
		t.Fatalf("cleanup = (%d, %v), want (2, nil)", deleted, err)
	}
	for _, eventID := range []string{deliveredEvent.ID, failedEvent.ID} {
		_, err := tdb.Store.GetWebhookEventByID(context.Background(), eventID)
		if !errors.Is(err, store.ErrorWebhookEventNotFound) {
			t.Fatalf("expected event %q to be deleted, got %v", eventID, err)
		}
	}
}

func newTestWorker(tdb *testutil.TestDB, server *httptest.Server) *Worker {
	return NewWorker(
		tdb.Store,
		NewSender(server.URL, "secret", server.Client()),
		nil,
		testWorkerConfig(),
	)
}

func runWorkerOnce(t *testing.T, worker *Worker, now time.Time) {
	t.Helper()
	processed, err := worker.RunOnce(context.Background(), now)
	if err != nil || !processed {
		t.Fatalf("RunOnce = (%v, %v), want (true, nil)", processed, err)
	}
}

func testWorkerConfig() WorkerConfig {
	return WorkerConfig{
		WorkerCount:          1,
		PollInterval:         time.Second,
		MaxDeliveryAttempts:  3,
		ProcessingStaleAfter: 2 * time.Minute,
		StaleReaperInterval:  time.Minute,
		DeliveredRetention:   24 * time.Hour,
		FailedRetention:      30 * 24 * time.Hour,
		CleanupInterval:      time.Hour,
		MaintenanceBatchSize: 1000,
	}
}

func getWebhookEvent(t *testing.T, tdb *testutil.TestDB, eventID string) domain.WebhookEvent {
	t.Helper()
	event, err := tdb.Store.GetWebhookEventByID(context.Background(), eventID)
	if err != nil {
		t.Fatalf("GetWebhookEventByID failed: %v", err)
	}
	return event
}

func assertTimeNear(t *testing.T, got, want time.Time) {
	t.Helper()
	if delta := got.Sub(want); delta < -time.Millisecond || delta > time.Millisecond {
		t.Fatalf("expected time near %s, got %s", want, got)
	}
}

func deleteWebhookEvent(t *testing.T, tdb *testutil.TestDB, eventID string) {
	t.Helper()
	if _, err := tdb.Store.DB().Exec(`DELETE FROM webhook_events WHERE id = $1`, eventID); err != nil {
		t.Errorf("delete webhook event: %v", err)
	}
}
