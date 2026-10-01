package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/maintenance"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

func TestMaintenanceLeaseAcquisitionAndFencing(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	resetCleanupLease(t, tdb)

	firstOwner := uuid.New()
	first, acquired, err := tdb.Store.TryAcquireMaintenanceLease(ctx, store.CleanupLeaseName, firstOwner, time.Minute)
	if err != nil || !acquired {
		t.Fatalf("first acquisition = (%t, %v)", acquired, err)
	}
	defer func() { _, _ = tdb.Store.ReleaseMaintenanceLease(context.Background(), first) }()

	if _, acquired, err := tdb.Store.TryAcquireMaintenanceLease(ctx, store.CleanupLeaseName, uuid.New(), time.Minute); err != nil || acquired {
		t.Fatalf("contending acquisition = (%t, %v), want skipped", acquired, err)
	}

	called := false
	rows, more, owned, err := tdb.Store.RunMaintenanceBatch(ctx, first, func(context.Context) (int64, bool, error) {
		called = true
		return 3, false, nil
	})
	if err != nil || !owned || !called || rows != 3 || more {
		t.Fatalf("owned batch = (rows=%d, more=%t, owned=%t, called=%t, err=%v)", rows, more, owned, called, err)
	}

	released, err := tdb.Store.ReleaseMaintenanceLease(ctx, first)
	if err != nil || !released {
		t.Fatalf("release = (%t, %v)", released, err)
	}
	second, acquired, err := tdb.Store.TryAcquireMaintenanceLease(ctx, store.CleanupLeaseName, uuid.New(), time.Minute)
	if err != nil || !acquired {
		t.Fatalf("second acquisition = (%t, %v)", acquired, err)
	}
	defer func() { _, _ = tdb.Store.ReleaseMaintenanceLease(context.Background(), second) }()

	called = false
	_, _, owned, err = tdb.Store.RunMaintenanceBatch(ctx, first, func(context.Context) (int64, bool, error) {
		called = true
		return 0, false, nil
	})
	if err != nil || owned || called {
		t.Fatalf("stale fenced batch = (owned=%t, called=%t, err=%v)", owned, called, err)
	}
}

func TestMaintenanceLeaseHasOneConcurrentWinner(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	resetCleanupLease(t, tdb)

	type result struct {
		lease    store.MaintenanceLease
		acquired bool
		err      error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			lease, acquired, err := tdb.Store.TryAcquireMaintenanceLease(
				context.Background(), store.CleanupLeaseName, uuid.New(), time.Minute,
			)
			results <- result{lease: lease, acquired: acquired, err: err}
		}()
	}
	close(start)
	workers.Wait()
	close(results)

	winners := 0
	var winningLease store.MaintenanceLease
	for got := range results {
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.acquired {
			winners++
			winningLease = got.lease
		}
	}
	if winners != 1 {
		t.Fatalf("concurrent lease winners = %d, want 1", winners)
	}
	if _, err := tdb.Store.ReleaseMaintenanceLease(context.Background(), winningLease); err != nil {
		t.Fatal(err)
	}
}

func TestMaintenanceLeaseExpiredOwnerIsFenced(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	resetCleanupLease(t, tdb)

	first, acquired, err := tdb.Store.TryAcquireMaintenanceLease(
		ctx, store.CleanupLeaseName, uuid.New(), time.Minute,
	)
	if err != nil || !acquired {
		t.Fatalf("first acquisition = (%t, %v)", acquired, err)
	}
	if _, err := tdb.Store.DB().ExecContext(ctx, `
		UPDATE authara.maintenance_leases
		SET lease_until = now() - interval '1 second'
		WHERE name = 'cleanup'
	`); err != nil {
		t.Fatal(err)
	}

	second, acquired, err := tdb.Store.TryAcquireMaintenanceLease(
		ctx, store.CleanupLeaseName, uuid.New(), time.Minute,
	)
	if err != nil || !acquired {
		t.Fatalf("takeover acquisition = (%t, %v)", acquired, err)
	}
	defer func() { _, _ = tdb.Store.ReleaseMaintenanceLease(context.Background(), second) }()

	called := false
	_, _, owned, err := tdb.Store.RunMaintenanceBatch(ctx, first, func(context.Context) (int64, bool, error) {
		called = true
		return 0, false, nil
	})
	if err != nil || owned || called {
		t.Fatalf("expired owner batch = (owned=%t, called=%t, err=%v)", owned, called, err)
	}
}

func TestMaintenanceBatchRollsBackOnFailure(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	resetCleanupLease(t, tdb)

	lease, acquired, err := tdb.Store.TryAcquireMaintenanceLease(
		ctx, store.CleanupLeaseName, uuid.New(), time.Minute,
	)
	if err != nil || !acquired {
		t.Fatalf("lease acquisition = (%t, %v)", acquired, err)
	}
	defer func() { _, _ = tdb.Store.ReleaseMaintenanceLease(context.Background(), lease) }()

	event, err := tdb.Store.CreateSecurityEvent(ctx, domain.SecurityEvent{
		Type:      domain.SecurityEventSessionLogout,
		Outcome:   domain.SecurityEventOutcomeSuccess,
		ActorType: domain.SecurityEventActorSystem,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = tdb.Store.DB().ExecContext(context.Background(), `DELETE FROM authara.security_events WHERE id = $1`, event.ID)
	}()
	if _, err := tdb.Store.DB().ExecContext(ctx, `
		UPDATE authara.security_events SET created_at = '1900-01-01' WHERE id = $1
	`, event.ID); err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("force rollback")
	rows, _, owned, err := tdb.Store.RunMaintenanceBatch(ctx, lease, func(batchCtx context.Context) (int64, bool, error) {
		deleted, deleteErr := tdb.Store.DeleteSecurityEventsBefore(batchCtx, time.Date(1901, 1, 1, 0, 0, 0, 0, time.UTC), 1)
		if deleteErr != nil {
			return 0, false, deleteErr
		}
		return deleted, false, wantErr
	})
	if !owned || !errors.Is(err, wantErr) || rows != 0 {
		t.Fatalf("failed batch = (rows=%d, owned=%t, err=%v)", rows, owned, err)
	}

	var count int
	if err := tdb.Store.DB().QueryRowContext(ctx, `
		SELECT count(*) FROM authara.security_events WHERE id = $1
	`, event.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("security event count after rollback = %d, want 1", count)
	}
}

func TestMaintenanceCoordinatorReleasesLeaseOnCancellation(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	resetCleanupLease(t, tdb)

	coordinator, err := maintenance.New(tdb.Store, nil, nil, []maintenance.Job{{
		Name:     "security_events",
		Interval: time.Hour,
		RunBatch: func(context.Context, time.Time) (int64, bool, error) {
			return 0, false, nil
		},
	}}, maintenance.Config{
		LeaseDuration:        time.Second,
		RenewInterval:        200 * time.Millisecond,
		RetryInterval:        20 * time.Millisecond,
		ContinuationInterval: 20 * time.Millisecond,
		PassTimeout:          500 * time.Millisecond,
		MaxBatches:           1,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	coordinator.Run(ctx)
	deadline := time.Now().Add(time.Second)
	for {
		var held bool
		if err := tdb.Store.DB().QueryRow(`
			SELECT owner_id IS NOT NULL FROM authara.maintenance_leases WHERE name = 'cleanup'
		`).Scan(&held); err != nil {
			t.Fatal(err)
		}
		if held {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("coordinator did not acquire cleanup lease")
		}
		time.Sleep(5 * time.Millisecond)
	}

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := coordinator.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}

	var released bool
	if err := tdb.Store.DB().QueryRow(`
		SELECT owner_id IS NULL FROM authara.maintenance_leases WHERE name = 'cleanup'
	`).Scan(&released); err != nil {
		t.Fatal(err)
	}
	if !released {
		t.Fatal("cleanup lease remained owned after coordinator cancellation")
	}
}

func resetCleanupLease(t *testing.T, tdb *testutil.TestDB) {
	t.Helper()
	if _, err := tdb.Store.DB().ExecContext(context.Background(), `
		UPDATE authara.maintenance_leases
		SET owner_id = NULL, lease_until = NULL, updated_at = now()
		WHERE name = 'cleanup'
	`); err != nil {
		t.Fatal(err)
	}
}
