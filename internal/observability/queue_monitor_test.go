package observability

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/store"
)

func TestQueueMonitorRefreshesBothQueuesAndStops(t *testing.T) {
	now := time.Now().UTC()
	oldest := now.Add(-time.Minute)
	statsStore := &queueStatsStoreStub{
		email: store.QueueStats{
			States:        []store.QueueStateStats{{State: store.QueueStatePending, Count: 2, OldestCreatedAt: &oldest}},
			OldestReadyAt: &oldest,
		},
		webhook: store.QueueStats{States: []store.QueueStateStats{{State: store.QueueStateFailed, Count: 1}}},
	}
	metrics := New("test")
	monitor, err := NewQueueMonitor(statsStore, metrics, slog.New(slog.NewTextHandler(io.Discard, nil)), QueueMonitorConfig{
		Interval:          time.Hour,
		Timeout:           time.Second,
		EmailStaleAfter:   func() time.Duration { return 2 * time.Minute },
		WebhookStaleAfter: func() time.Duration { return 3 * time.Minute },
	})
	if err != nil {
		t.Fatal(err)
	}

	monitor.Run(context.Background())
	deadline := time.Now().Add(time.Second)
	for statsStore.calls() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if statsStore.calls() != 2 {
		t.Fatalf("snapshot calls = %d, want 2", statsStore.calls())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := monitor.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	select {
	case <-monitor.Done():
	default:
		t.Fatal("Shutdown returned before monitor stopped")
	}

	metricsText := scrape(t, metrics)
	assertContains(t, metricsText, `authara_queue_jobs{queue="email",state="pending"} 2`)
	assertContains(t, metricsText, `authara_queue_jobs{queue="webhook",state="failed"} 1`)
}

func TestQueueMonitorRecordsIndependentSnapshotFailures(t *testing.T) {
	statsStore := &queueStatsStoreStub{
		emailErr: errors.New("database unavailable"),
		webhook:  store.QueueStats{States: []store.QueueStateStats{{State: store.QueueStatePending, Count: 1}}},
	}
	metrics := New("test")
	monitor, err := NewQueueMonitor(statsStore, metrics, slog.New(slog.NewTextHandler(io.Discard, nil)), QueueMonitorConfig{
		EmailStaleAfter:   func() time.Duration { return time.Minute },
		WebhookStaleAfter: func() time.Duration { return time.Minute },
	})
	if err != nil {
		t.Fatal(err)
	}

	monitor.refresh(context.Background())

	metricsText := scrape(t, metrics)
	assertContains(t, metricsText, `authara_queue_snapshot_refreshes_total{queue="email",result="failed"} 1`)
	assertContains(t, metricsText, `authara_queue_snapshot_refreshes_total{queue="webhook",result="succeeded"} 1`)
	if strings.Contains(metricsText, "database unavailable") {
		t.Fatal("metric output exposed an error message")
	}
}

type queueStatsStoreStub struct {
	mu         sync.Mutex
	email      store.QueueStats
	webhook    store.QueueStats
	emailErr   error
	webhookErr error
	callCount  int
}

func (s *queueStatsStoreStub) EmailQueueStats(context.Context, time.Time, time.Time) (store.QueueStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callCount++
	return s.email, s.emailErr
}

func (s *queueStatsStoreStub) WebhookQueueStats(context.Context, time.Time, time.Time) (store.QueueStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callCount++
	return s.webhook, s.webhookErr
}

func (s *queueStatsStoreStub) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callCount
}
