package webhook

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/authara-org/authara/internal/logging"
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
	MaintenanceBatchSize int
	Metrics              WorkerMetrics
	Policy               func() WorkerPolicy
}

type WorkerPolicy struct {
	MaxDeliveryAttempts  int
	ProcessingStaleAfter time.Duration
	DeliveredRetention   time.Duration
	FailedRetention      time.Duration
	MaintenanceBatchSize int
}

type WorkerMetrics interface {
	ObserveBackgroundJob(worker, outcome string, duration time.Duration)
}

type Worker struct {
	store   *store.Store
	sender  *Sender
	logger  *logging.Logger
	metrics WorkerMetrics
	cfg     WorkerConfig
	policy  func() WorkerPolicy
	wake    chan struct{}

	lifecycleMu sync.Mutex
	started     bool
	stopClaims  context.CancelFunc
	stopWork    context.CancelFunc
	workers     sync.WaitGroup
	done        chan struct{}
}

const webhookForcedShutdownGrace = time.Second
const webhookListenerReconnectDelay = time.Second

func NewWorker(store *store.Store, sender *Sender, logger *logging.Logger, cfg WorkerConfig) *Worker {
	if logger == nil {
		logger = logging.Discard()
	}
	policy := cfg.Policy
	if policy == nil {
		policy = func() WorkerPolicy {
			return WorkerPolicy{
				MaxDeliveryAttempts: cfg.MaxDeliveryAttempts, ProcessingStaleAfter: cfg.ProcessingStaleAfter,
				DeliveredRetention: cfg.DeliveredRetention, FailedRetention: cfg.FailedRetention,
				MaintenanceBatchSize: cfg.MaintenanceBatchSize,
			}
		}
	}
	return &Worker{
		store: store, sender: sender, logger: logger, metrics: cfg.Metrics, cfg: cfg, policy: policy,
		wake: make(chan struct{}, max(1, cfg.WorkerCount)), done: make(chan struct{}),
	}
}

func (w *Worker) Run(ctx context.Context) {
	w.lifecycleMu.Lock()
	if w.started {
		w.lifecycleMu.Unlock()
		return
	}
	w.started = true
	claimCtx, stopClaims := context.WithCancel(ctx)
	workCtx, stopWork := context.WithCancel(context.Background())
	w.stopClaims = stopClaims
	w.stopWork = stopWork

	for i := range w.cfg.WorkerCount {
		w.workers.Add(1)
		go func(workerID int) {
			defer w.workers.Done()
			w.run(claimCtx, workCtx, workerID)
		}(i + 1)
	}
	w.workers.Add(1)
	go func() {
		defer w.workers.Done()
		w.listenForReadyEvents(claimCtx)
	}()
	w.workers.Add(1)
	go func() {
		defer w.workers.Done()
		w.runMaintenance(claimCtx)
	}()
	go func() {
		w.workers.Wait()
		close(w.done)
	}()
	w.lifecycleMu.Unlock()
}

func (w *Worker) Shutdown(ctx context.Context) error {
	w.lifecycleMu.Lock()
	if !w.started {
		w.lifecycleMu.Unlock()
		return nil
	}
	stopClaims := w.stopClaims
	stopWork := w.stopWork
	done := w.done
	w.lifecycleMu.Unlock()

	stopClaims()
	force, stopForce := webhookShutdownForce(ctx, webhookForcedShutdownGrace)
	defer stopForce()
	select {
	case <-done:
		stopWork()
		return nil
	case <-force:
		stopWork()
	case <-ctx.Done():
		stopWork()
		return fmt.Errorf("%w: webhook workers did not stop before the shutdown deadline", ctx.Err())
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%w: webhook workers did not stop before the shutdown deadline", ctx.Err())
	}
}

func (w *Worker) Done() <-chan struct{} {
	return w.done
}

func webhookShutdownForce(ctx context.Context, reserve time.Duration) (<-chan time.Time, func()) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, func() {}
	}
	wait := time.Until(deadline.Add(-reserve))
	if wait < 0 {
		wait = 0
	}
	timer := time.NewTimer(wait)
	return timer.C, func() { timer.Stop() }
}

func (w *Worker) run(claimCtx, workCtx context.Context, _ int) {
	for {
		select {
		case <-claimCtx.Done():
			return
		default:
		}

		processed, err := w.runOnce(claimCtx, workCtx, time.Now().UTC(), w.wakeOne)
		if err != nil {
			if claimCtx.Err() != nil {
				return
			}
			w.logger.Log(claimCtx, logging.LevelError, logging.EventWorkerOutcome,
				logging.WorkerField(logging.WorkerWebhook),
				logging.OperationField(logging.OperationClaim),
				logging.OutcomeField(logging.OutcomeFailed),
				logging.ErrorCodeField(logging.ErrorStorageUnavailable),
			)
		}
		if processed && err == nil {
			continue
		}

		select {
		case <-claimCtx.Done():
			return
		case <-w.wake:
		case <-time.After(w.cfg.PollInterval):
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context, now time.Time) (bool, error) {
	return w.runOnce(ctx, ctx, now, nil)
}

func (w *Worker) runOnce(claimCtx, workCtx context.Context, now time.Time, onClaim func()) (bool, error) {
	policy := w.policy()
	pollStarted := time.Now()
	event, err := w.store.ClaimNextWebhookEvent(claimCtx, now)
	if err != nil {
		if errors.Is(err, store.ErrorWebhookEventNotFound) {
			w.observePoll("empty", pollStarted)
			return false, nil
		}
		if claimCtx.Err() == nil {
			w.observePoll("failed", pollStarted)
		}
		return false, err
	}
	w.observePoll("claimed", pollStarted)
	if onClaim != nil {
		onClaim()
	}
	started := time.Now()

	retryable, err := w.sender.sendOnce(workCtx, EventType(event.EventType), event.ID, event.Payload)
	if err != nil {
		processingStartedAt := *event.ProcessingStartedAt
		if retryable && event.AttemptCount < policy.MaxDeliveryAttempts {
			nextAttemptAt := now.Add(deliveryRetryDelay(event.AttemptCount))
			if requeueErr := w.store.RequeueWebhookEvent(
				workCtx,
				event.ID,
				processingStartedAt,
				err.Error(),
				nextAttemptAt,
			); requeueErr != nil {
				w.observeJob("error", started)
				return true, fmt.Errorf("requeue webhook event: %w", requeueErr)
			}
			w.logger.Log(workCtx, logging.LevelWarn, logging.EventWebhookDelivery,
				logging.OperationField(logging.OperationWebhookDeliver),
				logging.OutcomeField(logging.OutcomeRetryScheduled),
				logging.ErrorCodeField(logging.ErrorDeliveryFailed),
				logging.Attempt(event.AttemptCount),
				logging.Retryable(true),
			)
			w.observeJob("retried", started)
			return true, nil
		}

		if markErr := w.store.MarkWebhookEventFailed(workCtx, event.ID, processingStartedAt, err.Error()); markErr != nil {
			w.observeJob("error", started)
			return true, fmt.Errorf("mark failed webhook event: %w", markErr)
		}
		w.logger.Log(workCtx, logging.LevelError, logging.EventWebhookDelivery,
			logging.OperationField(logging.OperationWebhookDeliver),
			logging.OutcomeField(logging.OutcomeExhausted),
			logging.ErrorCodeField(logging.ErrorAttemptsExhausted),
			logging.Attempt(event.AttemptCount),
			logging.Retryable(false),
		)
		w.observeJob("failed", started)
		return true, nil
	}

	if err := w.store.MarkWebhookEventDelivered(workCtx, event.ID, *event.ProcessingStartedAt, now); err != nil {
		w.observeJob("error", started)
		return true, err
	}
	w.logger.Log(workCtx, logging.LevelInfo, logging.EventWebhookDelivery,
		logging.OperationField(logging.OperationWebhookDeliver),
		logging.OutcomeField(logging.OutcomeSucceeded),
		logging.Attempt(event.AttemptCount),
	)
	w.observeJob("succeeded", started)
	return true, nil
}

func (w *Worker) wakeOne() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *Worker) listenForReadyEvents(ctx context.Context) {
	if w.store == nil {
		<-ctx.Done()
		return
	}
	for {
		_ = w.store.ListenForWebhookEvents(ctx, w.wakeOne)
		if ctx.Err() != nil {
			return
		}
		w.logger.Log(ctx, logging.LevelError, logging.EventWorkerOutcome,
			logging.WorkerField(logging.WorkerWebhook),
			logging.OperationField(logging.OperationWebhookListen),
			logging.OutcomeField(logging.OutcomeFailed),
			logging.ErrorCodeField(logging.ErrorDependencyUnavailable),
		)
		select {
		case <-ctx.Done():
			return
		case <-time.After(webhookListenerReconnectDelay):
		}
	}
}

func (w *Worker) observeJob(outcome string, started time.Time) {
	if w.metrics != nil {
		w.metrics.ObserveBackgroundJob("webhook", outcome, time.Since(started))
	}
}

func (w *Worker) observePoll(result string, started time.Time) {
	if metrics, ok := w.metrics.(interface {
		ObserveBackgroundPoll(worker, result string, duration time.Duration)
	}); ok {
		metrics.ObserveBackgroundPoll("webhook", result, time.Since(started))
	}
}

func deliveryRetryDelay(attempt int) time.Duration {
	if attempt == 1 {
		return 30 * time.Second
	}
	return 2 * time.Minute
}

func (w *Worker) reapStale(ctx context.Context, now time.Time) (store.ReapResult, error) {
	policy := w.policy()
	var total store.ReapResult
	for {
		result, err := w.store.ReapStaleWebhookEvents(
			ctx,
			now.Add(-policy.ProcessingStaleAfter),
			now,
			policy.MaxDeliveryAttempts,
			policy.MaintenanceBatchSize,
		)
		if err != nil {
			return total, err
		}
		total.Retried += result.Retried
		total.Failed += result.Failed
		if result.Total() < int64(policy.MaintenanceBatchSize) {
			return total, nil
		}
	}
}

func (w *Worker) CleanupExpiredEventsBatch(ctx context.Context, now time.Time) (int64, bool, error) {
	policy := w.policy()
	deleted, err := w.store.DeleteExpiredWebhookEvents(
		ctx,
		now.Add(-policy.DeliveredRetention),
		now.Add(-policy.FailedRetention),
		policy.MaintenanceBatchSize,
	)
	return deleted, deleted == int64(policy.MaintenanceBatchSize), err
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
	defer reaper.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-reaper.C:
			started := time.Now()
			result, err := w.reapStale(ctx, now.UTC())
			if err != nil {
				if ctx.Err() == nil {
					w.observeReaper("failed", started, result)
					w.logger.Log(ctx, logging.LevelError, logging.EventQueueOutcome,
						logging.QueueField(logging.QueueWebhook),
						logging.OperationField(logging.OperationRecover),
						logging.OutcomeField(logging.OutcomeFailed),
						logging.ErrorCodeField(logging.ErrorStorageUnavailable),
						logging.RetriedCount(result.Retried),
						logging.FailedCount(result.Failed),
					)
				}
			} else {
				w.observeReaper("succeeded", started, result)
				if result.Total() > 0 {
					w.logger.Log(ctx, logging.LevelWarn, logging.EventQueueOutcome,
						logging.QueueField(logging.QueueWebhook),
						logging.OperationField(logging.OperationRecover),
						logging.OutcomeField(logging.OutcomeRecovered),
						logging.RetriedCount(result.Retried),
						logging.FailedCount(result.Failed),
					)
				}
			}
		}
	}
}

func (w *Worker) observeReaper(result string, started time.Time, jobs store.ReapResult) {
	if metrics, ok := w.metrics.(interface {
		ObserveQueueReaper(queue, result string, duration time.Duration, retried, failed int64)
	}); ok {
		metrics.ObserveQueueReaper("webhook", result, time.Since(started), jobs.Retried, jobs.Failed)
	}
}
