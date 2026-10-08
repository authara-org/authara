package maintenance

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

func TestCoordinatorRunsCleanupOnlyForLeaseHolder(t *testing.T) {
	leaseStore := &fakeLeaseStore{}
	var mu sync.Mutex
	runs := map[string]int{}
	newCoordinator := func(name string) *Coordinator {
		coordinator, err := New(leaseStore, discardLogger(), nil, []Job{{
			Name:     "sessions_expired",
			Interval: time.Hour,
			RunBatch: func(context.Context, time.Time) (int64, bool, error) {
				mu.Lock()
				runs[name]++
				mu.Unlock()
				return 1, false, nil
			},
		}}, Config{
			LeaseDuration: 200 * time.Millisecond,
			RenewInterval: 50 * time.Millisecond,
			RetryInterval: 10 * time.Millisecond,
			PassTimeout:   100 * time.Millisecond,
			MaxBatches:    1,
		})
		if err != nil {
			t.Fatal(err)
		}
		return coordinator
	}

	first := newCoordinator("first")
	second := newCoordinator("second")
	ctx, cancel := context.WithCancel(context.Background())
	first.Run(ctx)
	second.Run(ctx)

	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for {
		mu.Lock()
		total := runs["first"] + runs["second"]
		mu.Unlock()
		if total == 1 {
			break
		}
		select {
		case <-deadline.C:
			t.Fatalf("cleanup runs = %#v, want one leader run", runs)
		case <-time.After(time.Millisecond):
		}
	}

	time.Sleep(30 * time.Millisecond)
	mu.Lock()
	total := runs["first"] + runs["second"]
	mu.Unlock()
	if total != 1 {
		t.Fatalf("cleanup runs = %#v, want exactly one", runs)
	}

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := first.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	if err := second.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
}

func TestCoordinatorBoundsBatchesAndReportsCommittedRows(t *testing.T) {
	leaseStore := &fakeLeaseStore{}
	metrics := &fakeMetrics{}
	coordinator, err := New(leaseStore, discardLogger(), metrics, nil, Config{
		LeaseDuration: time.Second,
		RenewInterval: 200 * time.Millisecond,
		RetryInterval: time.Second,
		PassTimeout:   500 * time.Millisecond,
		MaxBatches:    3,
	})
	if err != nil {
		t.Fatal(err)
	}
	lease, acquired, err := leaseStore.TryAcquireMaintenanceLease(
		context.Background(), store.CleanupLeaseName, coordinator.ownerID, time.Second,
	)
	if err != nil || !acquired {
		t.Fatalf("acquire lease = (%t, %v)", acquired, err)
	}

	calls := 0
	outcome := coordinator.runJob(context.Background(), lease, Job{
		Name:     "sessions_expired",
		Interval: time.Hour,
		RunBatch: func(context.Context, time.Time) (int64, bool, error) {
			calls++
			return 7, true, nil
		},
	})
	if outcome != jobRunIncomplete {
		t.Fatalf("bounded pass outcome = %q, want %q", outcome, jobRunIncomplete)
	}
	if calls != 3 {
		t.Fatalf("batch calls = %d, want 3", calls)
	}
	if metrics.lastJob != "sessions_expired" || metrics.lastOutcome != "incomplete" || metrics.lastRows != 21 {
		t.Fatalf("maintenance metric = (%q, %q, %d)", metrics.lastJob, metrics.lastOutcome, metrics.lastRows)
	}
}

func TestCoordinatorReportsTimeBudgetAsIncomplete(t *testing.T) {
	leaseStore := &fakeLeaseStore{}
	metrics := &fakeMetrics{}
	coordinator, err := New(leaseStore, discardLogger(), metrics, nil, Config{
		LeaseDuration: time.Second,
		RenewInterval: 200 * time.Millisecond,
		RetryInterval: time.Second,
		PassTimeout:   10 * time.Millisecond,
		MaxBatches:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	lease, acquired, err := leaseStore.TryAcquireMaintenanceLease(
		context.Background(), store.CleanupLeaseName, coordinator.ownerID, time.Second,
	)
	if err != nil || !acquired {
		t.Fatalf("acquire lease = (%t, %v)", acquired, err)
	}

	outcome := coordinator.runJob(context.Background(), lease, Job{
		Name:     "security_events",
		Interval: time.Hour,
		RunBatch: func(ctx context.Context, _ time.Time) (int64, bool, error) {
			<-ctx.Done()
			return 0, true, ctx.Err()
		},
	})
	if outcome != jobRunIncomplete {
		t.Fatalf("timed-out pass outcome = %q, want %q", outcome, jobRunIncomplete)
	}
	if metrics.lastOutcome != "incomplete" {
		t.Fatalf("timed-out pass metric outcome = %q, want incomplete", metrics.lastOutcome)
	}
}

func TestCoordinatorStopsWhenFencingCheckFails(t *testing.T) {
	leaseStore := &fakeLeaseStore{}
	coordinator, err := New(leaseStore, discardLogger(), nil, nil, Config{
		LeaseDuration: time.Second,
		RenewInterval: 200 * time.Millisecond,
		RetryInterval: time.Second,
		PassTimeout:   500 * time.Millisecond,
		MaxBatches:    1,
	})
	if err != nil {
		t.Fatal(err)
	}

	run := false
	outcome := coordinator.runJob(context.Background(), store.MaintenanceLease{
		Name: store.CleanupLeaseName, OwnerID: uuid.New(), Generation: 99,
	}, Job{
		Name: "sessions_expired", Interval: time.Hour,
		RunBatch: func(context.Context, time.Time) (int64, bool, error) {
			run = true
			return 0, false, nil
		},
	})
	if outcome != jobRunCanceled {
		t.Fatalf("stale owner outcome = %q, want %q", outcome, jobRunCanceled)
	}
	if run {
		t.Fatal("cleanup batch ran without owning the lease")
	}
}

func TestCoordinatorQuicklyContinuesAnIncompleteJob(t *testing.T) {
	leaseStore := &fakeLeaseStore{}
	var mu sync.Mutex
	calls := 0
	ctx, cancel := context.WithCancel(context.Background())
	coordinator, err := New(leaseStore, discardLogger(), nil, []Job{{
		Name:     "security_events",
		Interval: time.Hour,
		RunBatch: func(context.Context, time.Time) (int64, bool, error) {
			mu.Lock()
			calls++
			if calls == 2 {
				cancel()
			}
			mu.Unlock()
			return 1, true, nil
		},
	}}, Config{
		LeaseDuration:        time.Second,
		RenewInterval:        200 * time.Millisecond,
		RetryInterval:        time.Second,
		ContinuationInterval: 10 * time.Millisecond,
		PassTimeout:          500 * time.Millisecond,
		MaxBatches:           1,
	})
	if err != nil {
		t.Fatal(err)
	}

	coordinator.Run(ctx)
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("incomplete cleanup was not continued promptly")
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := coordinator.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Fatalf("cleanup calls = %d, want 2 prompt continuation passes", calls)
	}
}

func TestCoordinatorDoesNotReportCanceledLeaseClaimAsFailure(t *testing.T) {
	leaseStore := &cancelingLeaseStore{fakeLeaseStore: &fakeLeaseStore{}, started: make(chan struct{})}
	metrics := &leaseMetrics{}
	coordinator, err := New(leaseStore, discardLogger(), metrics, nil, Config{
		LeaseDuration: time.Second,
		RenewInterval: 200 * time.Millisecond,
		RetryInterval: time.Second,
		PassTimeout:   500 * time.Millisecond,
		MaxBatches:    1,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	coordinator.Run(ctx)
	<-leaseStore.started
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := coordinator.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	if outcomes := metrics.snapshot(); len(outcomes) != 0 {
		t.Fatalf("lease outcomes during shutdown = %v, want none", outcomes)
	}
}

type fakeLeaseStore struct {
	mu    sync.Mutex
	lease store.MaintenanceLease
}

type cancelingLeaseStore struct {
	*fakeLeaseStore
	started chan struct{}
}

func (s *cancelingLeaseStore) TryAcquireMaintenanceLease(
	ctx context.Context, _ string, _ uuid.UUID, _ time.Duration,
) (store.MaintenanceLease, bool, error) {
	close(s.started)
	<-ctx.Done()
	return store.MaintenanceLease{}, false, ctx.Err()
}

func (s *fakeLeaseStore) TryAcquireMaintenanceLease(
	_ context.Context, name string, ownerID uuid.UUID, duration time.Duration,
) (store.MaintenanceLease, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lease.OwnerID != uuid.Nil && s.lease.OwnerID != ownerID && time.Now().Before(s.lease.LeaseUntil) {
		return store.MaintenanceLease{}, false, nil
	}
	s.lease = store.MaintenanceLease{
		Name: name, OwnerID: ownerID, LeaseUntil: time.Now().Add(duration), Generation: s.lease.Generation + 1,
	}
	return s.lease, true, nil
}

func (s *fakeLeaseStore) RenewMaintenanceLease(
	_ context.Context, lease store.MaintenanceLease, duration time.Duration,
) (store.MaintenanceLease, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lease.OwnerID != lease.OwnerID || s.lease.Generation != lease.Generation || time.Now().After(s.lease.LeaseUntil) {
		return store.MaintenanceLease{}, false, nil
	}
	s.lease.LeaseUntil = time.Now().Add(duration)
	return s.lease, true, nil
}

func (s *fakeLeaseStore) ReleaseMaintenanceLease(_ context.Context, lease store.MaintenanceLease) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lease.OwnerID != lease.OwnerID || s.lease.Generation != lease.Generation {
		return false, nil
	}
	s.lease.OwnerID = uuid.Nil
	s.lease.LeaseUntil = time.Time{}
	return true, nil
}

func (s *fakeLeaseStore) RunMaintenanceBatch(
	ctx context.Context,
	lease store.MaintenanceLease,
	batch func(context.Context) (int64, bool, error),
) (int64, bool, bool, error) {
	s.mu.Lock()
	owned := s.lease.OwnerID == lease.OwnerID &&
		s.lease.Generation == lease.Generation &&
		time.Now().Before(s.lease.LeaseUntil)
	s.mu.Unlock()
	if !owned {
		return 0, false, false, nil
	}
	rows, more, err := batch(ctx)
	return rows, more, true, err
}

type fakeMetrics struct {
	lastJob     string
	lastOutcome string
	lastRows    int64
}

type leaseMetrics struct {
	mu       sync.Mutex
	outcomes []string
}

func (m *leaseMetrics) ObserveMaintenanceLease(outcome string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outcomes = append(m.outcomes, outcome)
}

func (*leaseMetrics) SetMaintenanceLeader(bool) {}
func (*leaseMetrics) ObserveMaintenanceRun(string, string, time.Duration, int64) {
}

func (m *leaseMetrics) snapshot() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.outcomes...)
}

func (*fakeMetrics) ObserveMaintenanceLease(string) {}
func (*fakeMetrics) SetMaintenanceLeader(bool)      {}
func (m *fakeMetrics) ObserveMaintenanceRun(job, outcome string, _ time.Duration, rows int64) {
	m.lastJob = job
	m.lastOutcome = outcome
	m.lastRows = rows
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
