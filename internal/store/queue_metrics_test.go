package store_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
)

func TestQueueStatsClassifyEmailAndWebhookStates(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	t.Cleanup(func() { _ = tdb.Store.Close() })
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		txDB, ok := ctx.Value(store.DbKey).(*sql.Tx)
		if !ok {
			t.Fatal("test transaction missing from context")
		}
		if _, err := txDB.ExecContext(ctx, `SET TRANSACTION ISOLATION LEVEL REPEATABLE READ`); err != nil {
			t.Fatal(err)
		}

		now := time.Now().UTC()
		staleBefore := now.Add(-time.Minute)
		oldest := time.Date(1900, time.January, 1, 0, 0, 0, 0, time.UTC)
		oldestReady := oldest.Add(time.Minute)
		staleStarted := staleBefore.Add(-time.Minute)

		emailBefore, err := tdb.Store.EmailQueueStats(ctx, now, staleBefore)
		if err != nil {
			t.Fatalf("initial EmailQueueStats: %v", err)
		}
		webhookBefore, err := tdb.Store.WebhookQueueStats(ctx, now, staleBefore)
		if err != nil {
			t.Fatalf("initial WebhookQueueStats: %v", err)
		}

		if _, err := txDB.ExecContext(ctx, `
			INSERT INTO email_jobs (
				created_at, updated_at, to_email, template, status, attempt_count,
				next_attempt_at, delivery_deadline_at, processing_started_at, failed_at
			) VALUES
				($1, $1, $2, 'new_sign_in', 'pending', 0, $3, $4, NULL, NULL),
				($5, $5, $6, 'new_sign_in', 'pending', 1, $4, $4, NULL, NULL),
				($7, $7, $8, 'new_sign_in', 'processing', 1, $4, $4, $9, NULL),
				($10, $10, $11, 'new_sign_in', 'failed', 2, $4, $4, NULL, $10)
		`,
			oldest, fmt.Sprintf("queue-pending-%d@example.test", now.UnixNano()), oldestReady, now.Add(time.Hour),
			oldest.Add(time.Second), fmt.Sprintf("queue-retry-%d@example.test", now.UnixNano()),
			oldest.Add(2*time.Second), fmt.Sprintf("queue-processing-%d@example.test", now.UnixNano()), staleStarted,
			oldest.Add(3*time.Second), fmt.Sprintf("queue-failed-%d@example.test", now.UnixNano()),
		); err != nil {
			t.Fatalf("insert email queue fixtures: %v", err)
		}

		prefix := fmt.Sprintf("queue-metrics-%d", now.UnixNano())
		if _, err := txDB.ExecContext(ctx, `
			INSERT INTO webhook_events (
				id, created_at, updated_at, event_type, payload, status, attempt_count,
				next_attempt_at, processing_started_at
			) VALUES
				($1, $2, $2, 'test.queue.metrics', '{}', 'pending', 0, $3, NULL),
				($4, $5, $5, 'test.queue.metrics', '{}', 'pending', 1, $6, NULL),
				($7, $8, $8, 'test.queue.metrics', '{}', 'processing', 1, $6, $9),
				($10, $11, $11, 'test.queue.metrics', '{}', 'failed', 2, $6, NULL)
		`,
			prefix+"-pending", oldest, oldestReady,
			prefix+"-retry", oldest.Add(time.Second), now.Add(time.Hour),
			prefix+"-processing", oldest.Add(2*time.Second), staleStarted,
			prefix+"-failed", oldest.Add(3*time.Second),
		); err != nil {
			t.Fatalf("insert webhook queue fixtures: %v", err)
		}

		emailAfter, err := tdb.Store.EmailQueueStats(ctx, now, staleBefore)
		if err != nil {
			t.Fatalf("EmailQueueStats: %v", err)
		}
		assertQueueStatsDelta(t, emailBefore, emailAfter, oldest, oldestReady)

		webhookAfter, err := tdb.Store.WebhookQueueStats(ctx, now, staleBefore)
		if err != nil {
			t.Fatalf("WebhookQueueStats: %v", err)
		}
		assertQueueStatsDelta(t, webhookBefore, webhookAfter, oldest, oldestReady)
	})
}

func assertQueueStatsDelta(t *testing.T, before, after store.QueueStats, oldest, oldestReady time.Time) {
	t.Helper()
	wantStates := []string{store.QueueStatePending, store.QueueStateRetry, store.QueueStateProcessing, store.QueueStateFailed}
	if len(after.States) != len(wantStates) {
		t.Fatalf("queue states = %+v, want %v", after.States, wantStates)
	}
	for _, state := range wantStates {
		beforeState := queueState(before, state)
		afterState := queueState(after, state)
		if afterState.Count != beforeState.Count+1 {
			t.Fatalf("%s count = %d, want %d", state, afterState.Count, beforeState.Count+1)
		}
		wantOldest := oldestForState(oldest, state)
		if afterState.OldestCreatedAt == nil || !afterState.OldestCreatedAt.Equal(wantOldest) {
			t.Fatalf("%s oldest = %v, want %s", state, afterState.OldestCreatedAt, wantOldest)
		}
	}
	if after.Stuck != before.Stuck+1 {
		t.Fatalf("stuck jobs = %d, want %d", after.Stuck, before.Stuck+1)
	}
	if after.OldestReadyAt == nil || !after.OldestReadyAt.Equal(oldestReady) {
		t.Fatalf("oldest ready = %v, want %s", after.OldestReadyAt, oldestReady)
	}
}

func queueState(stats store.QueueStats, state string) store.QueueStateStats {
	for _, current := range stats.States {
		if current.State == state {
			return current
		}
	}
	return store.QueueStateStats{State: state}
}

func oldestForState(oldest time.Time, state string) time.Time {
	switch state {
	case store.QueueStateRetry:
		return oldest.Add(time.Second)
	case store.QueueStateProcessing:
		return oldest.Add(2 * time.Second)
	case store.QueueStateFailed:
		return oldest.Add(3 * time.Second)
	default:
		return oldest
	}
}
