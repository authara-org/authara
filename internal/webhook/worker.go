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
	WorkerCount  int
	PollInterval time.Duration
}

type Worker struct {
	store  *store.Store
	sender *Sender
	logger *slog.Logger
	cfg    WorkerConfig
}

func NewWorker(store *store.Store, sender *Sender, logger *slog.Logger, cfg WorkerConfig) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{store: store, sender: sender, logger: logger, cfg: cfg}
}

func (w *Worker) Run(ctx context.Context) {
	for i := range w.cfg.WorkerCount {
		go w.run(ctx, i+1)
	}
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

	err = w.sender.PublishPayload(ctx, EventType(event.EventType), event.ID, event.Payload)
	if err != nil {
		if markErr := w.store.MarkWebhookEventFailed(ctx, event.ID, err.Error()); markErr != nil {
			return true, fmt.Errorf("mark failed webhook event: %w", markErr)
		}
		w.logger.WarnContext(ctx, "webhook event failed",
			"event_id", event.ID,
			"event_type", event.EventType,
			"error", err,
		)
		return true, nil
	}

	if err := w.store.MarkWebhookEventDelivered(ctx, event.ID, now); err != nil {
		return true, err
	}
	w.logger.InfoContext(ctx, "webhook event delivered",
		"event_id", event.ID,
		"event_type", event.EventType,
	)
	return true, nil
}
