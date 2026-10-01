package store_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

func TestDeleteOperatorAuditEventsBeforeUsesRetentionCutoffAndBatchSize(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		txDB := ctx.Value(store.DbKey).(*sql.Tx)
		cutoff := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
		expiredIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
		retainedIDs := []uuid.UUID{uuid.New(), uuid.New()}

		for _, event := range []struct {
			id        uuid.UUID
			createdAt time.Time
		}{
			{id: expiredIDs[0], createdAt: cutoff.Add(-3 * time.Hour)},
			{id: expiredIDs[1], createdAt: cutoff.Add(-2 * time.Hour)},
			{id: expiredIDs[2], createdAt: cutoff.Add(-time.Hour)},
			{id: retainedIDs[0], createdAt: cutoff},
			{id: retainedIDs[1], createdAt: cutoff.Add(time.Hour)},
		} {
			if _, err := txDB.ExecContext(ctx, `
				INSERT INTO operator_audit_events (
					id, created_at, action, resource_type, resource_id
				) VALUES ($1, $2, 'test.action', 'test.resource', $3)
			`, event.id, event.createdAt, event.id.String()); err != nil {
				t.Fatal(err)
			}
		}

		deleted, err := tdb.Store.DeleteOperatorAuditEventsBefore(ctx, cutoff, 2)
		if err != nil {
			t.Fatal(err)
		}
		if deleted != 2 {
			t.Fatalf("first batch deleted = %d, want 2", deleted)
		}

		deleted, err = tdb.Store.DeleteOperatorAuditEventsBefore(ctx, cutoff, 2)
		if err != nil {
			t.Fatal(err)
		}
		if deleted != 1 {
			t.Fatalf("second batch deleted = %d, want 1", deleted)
		}

		var expiredRemaining int
		if err := txDB.QueryRowContext(ctx, `
			SELECT count(*)
			FROM operator_audit_events
			WHERE id = ANY($1)
		`, expiredIDs).Scan(&expiredRemaining); err != nil {
			t.Fatal(err)
		}
		if expiredRemaining != 0 {
			t.Fatalf("expired events remaining = %d, want 0", expiredRemaining)
		}

		var retained int
		if err := txDB.QueryRowContext(ctx, `
			SELECT count(*)
			FROM operator_audit_events
			WHERE id = ANY($1)
		`, retainedIDs).Scan(&retained); err != nil {
			t.Fatal(err)
		}
		if retained != len(retainedIDs) {
			t.Fatalf("retained events = %d, want %d", retained, len(retainedIDs))
		}
	})
}
