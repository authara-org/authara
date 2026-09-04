package webhook

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/authara-org/authara/internal/store"
)

type WorkerConfig struct {
	WorkerCount          int
	PollInterval         time.Duration
	MaxDeliveryAttempts  int
	ProcessingStaleAfter time.Duration
	StaleReaperInterval  time.Duration
	DeliveredRetention   time.Duration
	FailedRetention      time.Duration
	CleanupInterval      time.Duration
	MaintenanceBatchSize int
	Metrics              WorkerMetrics
}

type WorkerMetrics interface {
	ObserveBackgroundJob(worker, outcome string, duration time.Duration)
}

type Worker struct {
	store   *store.Store
	sender  *Sender
	logger  *slog.Logger
	metrics WorkerMetrics
	cfg     WorkerConfig
}

func NewWorker(store *store.Store, sender *Sender, logger *slog.Logger, cfg WorkerConfig) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{store: store, sender: sender, logger: logger, metrics: cfg.Metrics, cfg: cfg}
}

func (w *Worker) Run(ctx context.Context) {
	for i := range w.cfg.WorkerCount {
		go w.run(ctx, i+1)
	}
	go w.runMaintenance(ctx)
}

func (w *Worker) run(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		processed, err := w.RunOnce(ctx, time.Now().UTC())
		if err != nil {
			w.logger.ErrorContext(ctx, "webhook worker iteration failed",
				"worker_id", workerID,
				"error", err,
			)
		}
		if processed && err == nil {
			continue
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(w.cfg.PollInterval):
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context, now time.Time) (bool, error) {
	event, err := w.store.ClaimNextWebhookEvent(ctx, now)
	if err != nil {
		if errors.Is(err, store.ErrorWebhookEventNotFound) {
			return false, nil
		}
		return false, err
	}
	started := time.Now()

	retryable, err := w.sender.sendOnce(ctx, EventType(event.EventType), event.ID, event.Payload)
	if err != nil {
		processingStartedAt := *event.ProcessingStartedAt
		if retryable && event.AttemptCount < w.cfg.MaxDeliveryAttempts {
			nextAttemptAt := now.Add(deliveryRetryDelay(event.AttemptCount))
			if requeueErr := w.store.RequeueWebhookEvent(
				ctx,
				event.ID,
				processingStartedAt,
				err.Error(),
				nextAttemptAt,
			); requeueErr != nil {
				w.observeJob("error", started)
				return true, fmt.Errorf("requeue webhook event: %w", requeueErr)
			}
			w.logger.WarnContext(ctx, "webhook event retry scheduled",
				"event_id", event.ID,
				"event_type", event.EventType,
				"attempt", event.AttemptCount,
				"next_attempt_at", nextAttemptAt,
				"error", err,
			)
			w.observeJob("retried", started)
			return true, nil
		}

		if markErr := w.store.MarkWebhookEventFailed(ctx, event.ID, processingStartedAt, err.Error()); markErr != nil {
			w.observeJob("error", started)
			return true, fmt.Errorf("mark failed webhook event: %w", markErr)
		}
		w.logger.WarnContext(ctx, "webhook event failed",
			"event_id", event.ID,
			"event_type", event.EventType,
			"attempt", event.AttemptCount,
			"error", err,
		)
		w.observeJob("failed", started)
		return true, nil
	}

	if err := w.store.MarkWebhookEventDelivered(ctx, event.ID, *event.ProcessingStartedAt, now); err != nil {
		w.observeJob("error", started)
		return true, err
	}
	w.logger.InfoContext(ctx, "webhook event delivered",
		"event_id", event.ID,
		"event_type", event.EventType,
	)
	w.observeJob("succeeded", started)
	return true, nil
}

func (w *Worker) observeJob(outcome string, started time.Time) {
	if w.metrics != nil {
		w.metrics.ObserveBackgroundJob("webhook", outcome, time.Since(started))
	}
}

func deliveryRetryDelay(attempt int) time.Duration {
	if attempt == 1 {
		return 30 * time.Second
	}
	return 2 * time.Minute
}

func (w *Worker) reapStale(ctx context.Context, now time.Time) (int64, error) {
	return drainBatches(w.cfg.MaintenanceBatchSize, func() (int64, error) {
		return w.store.ReapStaleWebhookEvents(
			ctx,
			now.Add(-w.cfg.ProcessingStaleAfter),
			now,
			w.cfg.MaxDeliveryAttempts,
			w.cfg.MaintenanceBatchSize,
		)
	})
}

func (w *Worker) cleanup(ctx context.Context, now time.Time) (int64, error) {
	return drainBatches(w.cfg.MaintenanceBatchSize, func() (int64, error) {
		return w.store.DeleteExpiredWebhookEvents(
			ctx,
			now.Add(-w.cfg.DeliveredRetention),
			now.Add(-w.cfg.FailedRetention),
			w.cfg.MaintenanceBatchSize,
		)
	})
}

func drainBatches(batchSize int, deleteBatch func() (int64, error)) (int64, error) {
	var total int64
	for {
		deleted, err := deleteBatch()
		if err != nil {
			return total, err
		}
		total += deleted
		if deleted < int64(batchSize) {
			return total, nil
		}
	}
}

func (w *Worker) runMaintenance(ctx context.Context) {
	reaper := time.NewTicker(w.cfg.StaleReaperInterval)
	cleanup := time.NewTicker(w.cfg.CleanupInterval)
	defer reaper.Stop()
	defer cleanup.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-reaper.C:
			count, err := w.reapStale(ctx, now.UTC())
			if err != nil {
				w.logger.ErrorContext(ctx, "webhook stale reaper failed", "error", err)
			} else if count > 0 {
				w.logger.WarnContext(ctx, "stale webhook events reaped", "event_count", count)
			}
		case now := <-cleanup.C:
			count, err := w.cleanup(ctx, now.UTC())
			if err != nil {
				w.logger.ErrorContext(ctx, "webhook event cleanup failed", "error", err)
			} else if count > 0 {
				w.logger.InfoContext(ctx, "webhook events cleaned up", "event_count", count)
			}
		}
	}
}
