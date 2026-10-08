package observability

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/authara-org/authara/internal/store"
)

const (
	defaultQueueSnapshotInterval = 15 * time.Second
	defaultQueueSnapshotTimeout  = 2 * time.Second
)

type QueueStatsStore interface {
	EmailQueueStats(context.Context, time.Time, time.Time) (store.QueueStats, error)
	WebhookQueueStats(context.Context, time.Time, time.Time) (store.QueueStats, error)
}

type QueueMonitorConfig struct {
	Interval          time.Duration
	Timeout           time.Duration
	EmailStaleAfter   func() time.Duration
	WebhookStaleAfter func() time.Duration
}

type QueueMonitor struct {
	store   QueueStatsStore
	metrics *Service
	logger  *slog.Logger
	cfg     QueueMonitorConfig

	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	done    chan struct{}
}

func NewQueueMonitor(store QueueStatsStore, metrics *Service, logger *slog.Logger, cfg QueueMonitorConfig) (*QueueMonitor, error) {
	if store == nil {
		return nil, errors.New("queue metrics store is required")
	}
	if metrics == nil {
		return nil, errors.New("observability service is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Interval <= 0 {
		cfg.Interval = defaultQueueSnapshotInterval
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultQueueSnapshotTimeout
	}
	if cfg.EmailStaleAfter == nil || cfg.WebhookStaleAfter == nil {
		return nil, errors.New("queue stale-processing policies are required")
	}
	return &QueueMonitor{store: store, metrics: metrics, logger: logger, cfg: cfg, done: make(chan struct{})}, nil
}

func (m *QueueMonitor) Run(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return
	}
	monitorCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.started = true
	go func() {
		defer close(m.done)
		m.run(monitorCtx)
	}()
}

func (m *QueueMonitor) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return nil
	}
	m.cancel()
	done := m.done
	m.mu.Unlock()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *QueueMonitor) Done() <-chan struct{} { return m.done }

func (m *QueueMonitor) run(ctx context.Context) {
	m.refresh(ctx)
	ticker := time.NewTicker(m.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.refresh(ctx)
		}
	}
}

func (m *QueueMonitor) refresh(ctx context.Context) {
	now := time.Now().UTC()
	m.refreshQueue(ctx, "email", now, m.cfg.EmailStaleAfter(), m.store.EmailQueueStats)
	if ctx.Err() == nil {
		m.refreshQueue(ctx, "webhook", now, m.cfg.WebhookStaleAfter(), m.store.WebhookQueueStats)
	}
}

func (m *QueueMonitor) refreshQueue(
	ctx context.Context,
	queue string,
	now time.Time,
	staleAfter time.Duration,
	load func(context.Context, time.Time, time.Time) (store.QueueStats, error),
) {
	queryCtx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()
	stats, err := load(queryCtx, now, now.Add(-staleAfter))
	if err != nil {
		if ctx.Err() == nil {
			m.metrics.ObserveQueueSnapshot(queue, "failed", store.QueueStats{}, now)
			m.logger.ErrorContext(ctx, "queue metrics snapshot failed", "queue", queue, "error", err)
		}
		return
	}
	m.metrics.ObserveQueueSnapshot(queue, "succeeded", stats, now)
}
