package challenge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/store"
)

type WorkerConfig struct {
	WorkerCount          int
	PollInterval         time.Duration
	JobMaxAttempts       int
	RetryBaseDelay       time.Duration
	RetryMaxDelay        time.Duration
	ProcessingStaleAfter time.Duration
	StaleReaperInterval  time.Duration
	MaintenanceBatchSize int
	CleanupSentAfter     time.Duration
	CleanupFailedAfter   time.Duration
	CleanupInterval      time.Duration
	SendTimeout          time.Duration
	TransitionTimeout    time.Duration
	Policy               config.EmailPolicyReader
	Metrics              WorkerMetrics
	RandomFloat64        func() float64
}

type WorkerMetrics interface {
	ObserveBackgroundJob(worker, outcome string, duration time.Duration)
}

type EmailTemplateRenderer interface {
	Render(context.Context, domain.EmailTemplate, email.TemplateData) (email.Message, error)
}

type Worker struct {
	store     *store.Store
	codeSvc   *VerificationCodeService
	templates EmailTemplateRenderer
	sender    email.Sender
	logger    *slog.Logger
	metrics   WorkerMetrics
	cfg       WorkerConfig
	policy    config.EmailPolicyReader
	claimNext func(context.Context, time.Time) (domain.EmailJob, error)

	lifecycleMu sync.Mutex
	started     bool
	stopClaims  context.CancelFunc
	stopWork    context.CancelFunc
	workers     sync.WaitGroup
}

const (
	defaultEmailMaxAttempts          = 100
	defaultEmailRetryBaseDelay       = 30 * time.Second
	defaultEmailRetryMaxDelay        = 6 * time.Hour
	defaultEmailProcessingStaleAfter = 2 * time.Minute
	defaultEmailStaleReaperInterval  = time.Minute
	defaultEmailMaintenanceBatchSize = 1000
	defaultEmailTransitionTimeout    = 5 * time.Second
	emailForcedShutdownWait          = time.Second
)

func NewWorker(
	store *store.Store,
	codeSvc *VerificationCodeService,
	templates EmailTemplateRenderer,
	sender email.Sender,
	logger *slog.Logger,
	cfg WorkerConfig,
) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.JobMaxAttempts <= 0 {
		cfg.JobMaxAttempts = defaultEmailMaxAttempts
	}
	if cfg.RetryBaseDelay <= 0 {
		cfg.RetryBaseDelay = defaultEmailRetryBaseDelay
	}
	if cfg.RetryMaxDelay < cfg.RetryBaseDelay {
		cfg.RetryMaxDelay = defaultEmailRetryMaxDelay
	}
	if cfg.ProcessingStaleAfter <= 0 {
		cfg.ProcessingStaleAfter = defaultEmailProcessingStaleAfter
	}
	if cfg.ProcessingStaleAfter <= cfg.SendTimeout {
		cfg.ProcessingStaleAfter = cfg.SendTimeout + time.Minute
	}
	if cfg.StaleReaperInterval <= 0 {
		cfg.StaleReaperInterval = defaultEmailStaleReaperInterval
	}
	if cfg.MaintenanceBatchSize <= 0 {
		cfg.MaintenanceBatchSize = defaultEmailMaintenanceBatchSize
	}
	if cfg.TransitionTimeout <= 0 {
		cfg.TransitionTimeout = defaultEmailTransitionTimeout
	}
	if cfg.RandomFloat64 == nil {
		cfg.RandomFloat64 = rand.Float64
	}
	policy := cfg.Policy
	if policy == nil {
		policy = config.EmailPolicyReaderFunc(func() config.EmailPolicy {
			return config.EmailPolicy{
				JobMaxAttempts: cfg.JobMaxAttempts, CleanupSentAfter: cfg.CleanupSentAfter,
				CleanupFailedAfter: cfg.CleanupFailedAfter,
			}
		})
	}

	return &Worker{
		store:     store,
		codeSvc:   codeSvc,
		templates: templates,
		sender:    sender,
		logger:    logger,
		metrics:   cfg.Metrics,
		cfg:       cfg,
		policy:    policy,
		claimNext: store.ClaimNextEmailJob,
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
			w.runWorker(claimCtx, workCtx, workerID)
		}(i + 1)
	}

	if w.cfg.CleanupInterval > 0 {
		w.workers.Add(1)
		go func() {
			defer w.workers.Done()
			w.runCleanupLoop(claimCtx)
		}()
	}
	if w.cfg.StaleReaperInterval > 0 {
		w.workers.Add(1)
		go func() {
			defer w.workers.Done()
			w.runStaleReaperLoop(claimCtx)
		}()
	}
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
	w.lifecycleMu.Unlock()

	stopClaims()
	done := make(chan struct{})
	go func() {
		w.workers.Wait()
		close(done)
	}()

	select {
	case <-done:
		stopWork()
		return nil
	case <-ctx.Done():
		drainErr := ctx.Err()
		stopWork()

		// Keep the store available long enough for cancelled deliveries to
		// persist their final lease-fenced transition. A sender that ignores
		// cancellation cannot hold shutdown open indefinitely; its lease will
		// still be reclaimed by the stale-job reaper.
		forcedWait := time.NewTimer(w.cfg.TransitionTimeout + emailForcedShutdownWait)
		defer forcedWait.Stop()
		select {
		case <-done:
			return drainErr
		case <-forcedWait.C:
			return fmt.Errorf("%w: email workers did not stop after cancellation", drainErr)
		}
	}
}

func (w *Worker) runWorker(claimCtx, workCtx context.Context, workerID int) {
	for {
		select {
		case <-claimCtx.Done():
			return
		default:
		}

		processed, err := w.runOnce(claimCtx, workCtx, time.Now().UTC())
		if err != nil {
			if claimCtx.Err() != nil {
				return
			}
			w.logger.ErrorContext(claimCtx, "email worker iteration failed",
				"worker_id", workerID,
				"error", err,
			)

			select {
			case <-claimCtx.Done():
				return
			case <-time.After(w.cfg.PollInterval):
			}
			continue
		}

		if processed {
			continue
		}

		select {
		case <-claimCtx.Done():
			return
		case <-time.After(w.cfg.PollInterval):
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context, now time.Time) (bool, error) {
	return w.runOnce(ctx, ctx, now)
}

func (w *Worker) runOnce(claimCtx, workCtx context.Context, now time.Time) (bool, error) {
	policy := w.policy.CurrentEmail()
	job, err := w.claimNext(claimCtx, now)
	if err != nil {
		if errors.Is(err, store.ErrorEmailJobNotFound) {
			return false, nil
		}
		return false, err
	}
	started := time.Now()
	lease := *job.ProcessingStartedAt
	queueAge := max(now.Sub(job.CreatedAt), 0)

	if !job.DeliveryDeadlineAt.After(now) {
		err := fmt.Errorf("email delivery deadline exceeded at %s", job.DeliveryDeadlineAt.UTC().Format(time.RFC3339))
		transitionCtx, transitionCancel := w.transitionContext(claimCtx)
		defer transitionCancel()
		return true, w.failJob(transitionCtx, job, lease, err, "delivery_deadline_exceeded", now, started, queueAge)
	}

	sendCtx, cancel := context.WithTimeout(workCtx, w.cfg.SendTimeout)
	defer cancel()

	deliveryErr := w.processJob(sendCtx, job, now)
	transitionCtx, transitionCancel := w.transitionContext(claimCtx)
	defer transitionCancel()

	if deliveryErr != nil {
		failureClass, failureReason := email.ClassifyFailure(deliveryErr)
		if failureClass == email.FailurePermanent {
			return true, w.failJob(transitionCtx, job, lease, deliveryErr, failureReason, now, started, queueAge)
		}
		if job.AttemptCount >= policy.JobMaxAttempts {
			return true, w.failJob(transitionCtx, job, lease, deliveryErr, "attempts_exhausted", now, started, queueAge)
		}

		nextAttemptAt := now.Add(w.retryDelay(job.AttemptCount))
		if nextAttemptAt.After(job.DeliveryDeadlineAt) {
			nextAttemptAt = job.DeliveryDeadlineAt
		}
		if requeueErr := w.store.RequeueEmailJob(transitionCtx, job.ID, lease, deliveryErr.Error(), nextAttemptAt); requeueErr != nil {
			w.observeJob("error", started, queueAge)
			return true, fmt.Errorf("requeue email job: %w", requeueErr)
		}
		w.logger.WarnContext(transitionCtx, "email job retry scheduled",
			"job_id", job.ID,
			"template", job.Template,
			"attempt", job.AttemptCount,
			"failure_class", failureClass,
			"failure_reason", failureReason,
			"next_attempt_at", nextAttemptAt,
			"queue_age", queueAge,
			"error", deliveryErr,
		)
		w.observeJob("retried", started, queueAge)
		return true, nil
	}

	if err := w.store.MarkEmailJobSent(transitionCtx, job.ID, lease, now); err != nil {
		w.observeJob("error", started, queueAge)
		return true, fmt.Errorf("mark email job sent: %w", err)
	}

	if job.ChallengeID != nil {
		_ = w.store.SetChallengeLastSentAt(transitionCtx, *job.ChallengeID, now)
	}

	w.logger.InfoContext(transitionCtx, "email job sent",
		"job_id", job.ID,
		"template", job.Template,
		"attempt", job.AttemptCount,
		"queue_age", queueAge,
	)
	w.observeJob("succeeded", started, queueAge)

	return true, nil
}

func (w *Worker) transitionContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), w.cfg.TransitionTimeout)
}

func (w *Worker) failJob(
	ctx context.Context,
	job domain.EmailJob,
	lease time.Time,
	deliveryErr error,
	terminalReason string,
	now time.Time,
	started time.Time,
	queueAge time.Duration,
) error {
	if err := w.store.MarkEmailJobFailed(ctx, job.ID, lease, deliveryErr.Error(), terminalReason, now); err != nil {
		w.observeJob("error", started, queueAge)
		return fmt.Errorf("mark email job failed: %w", err)
	}
	w.logger.ErrorContext(ctx, "email job permanently failed",
		"job_id", job.ID,
		"template", job.Template,
		"attempt", job.AttemptCount,
		"terminal_reason", terminalReason,
		"queue_age", queueAge,
		"error", deliveryErr,
	)
	w.observeJob("failed", started, queueAge)
	return nil
}

func (w *Worker) retryDelay(attempt int) time.Duration {
	return emailRetryDelay(attempt, w.cfg.RetryBaseDelay, w.cfg.RetryMaxDelay, w.cfg.RandomFloat64())
}

func emailRetryDelay(attempt int, baseDelay, maxDelay time.Duration, random float64) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	nominal := baseDelay
	for current := 1; current < attempt && nominal < maxDelay; current++ {
		if nominal > maxDelay/2 {
			nominal = maxDelay
			break
		}
		nominal *= 2
	}
	if nominal > maxDelay {
		nominal = maxDelay
	}
	if random < 0 {
		random = 0
	}
	if random >= 1 {
		random = 0.999999999
	}
	half := nominal / 2
	return half + time.Duration(random*float64(nominal-half))
}

func (w *Worker) observeJob(outcome string, started time.Time, queueAge time.Duration) {
	if w.metrics != nil {
		w.metrics.ObserveBackgroundJob("email", outcome, time.Since(started))
		if queueMetrics, ok := w.metrics.(interface {
			ObserveEmailQueueAge(outcome string, age time.Duration)
		}); ok {
			queueMetrics.ObserveEmailQueueAge(outcome, queueAge)
		}
	}
}

func (w *Worker) processJob(ctx context.Context, job domain.EmailJob, now time.Time) error {
	if w.templates == nil {
		return email.PermanentError("configuration_error", errors.New("email template renderer is not configured"))
	}

	templateData := make(email.TemplateData)
	if len(job.TemplateData) > 0 {
		if err := json.Unmarshal(job.TemplateData, &templateData); err != nil {
			return email.PermanentError("invalid_job_data", fmt.Errorf("decode email template data for %q: %w", job.Template, err))
		}
	}

	switch job.Template {
	case domain.EmailTemplateSignupCode,
		domain.EmailTemplatePasswordResetCode,
		domain.EmailTemplateEmailChangeCode:
		code, err := w.verificationCodeForJob(ctx, job, now)
		if err != nil {
			return err
		}
		templateData[email.TemplateVariableCode] = code

	default:
		if err := email.ValidateTemplate(job.Template); err != nil {
			return email.PermanentError("unsupported_template", fmt.Errorf("unsupported email template: %w", err))
		}
	}

	msg, err := w.templates.Render(ctx, job.Template, templateData)
	if err != nil {
		wrapped := fmt.Errorf("render email template %q: %w", job.Template, err)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return email.TransientError("delivery_cancelled", wrapped)
		}
		if isPermanentTemplateError(err) {
			return email.PermanentError("template_render_failed", wrapped)
		}
		return wrapped
	}
	if w.sender == nil {
		return email.PermanentError("configuration_error", errors.New("email sender is not configured"))
	}
	return w.sender.Send(ctx, job.ToEmail, msg)
}

func isPermanentTemplateError(err error) bool {
	return errors.Is(err, email.ErrEmptyTemplatePart) ||
		errors.Is(err, email.ErrMalformedTemplate) ||
		errors.Is(err, email.ErrUnknownTemplateVariable) ||
		errors.Is(err, email.ErrMissingTemplatePlaceholder) ||
		errors.Is(err, email.ErrInvalidTemplateSubject) ||
		errors.Is(err, email.ErrUnknownTemplate) ||
		errors.Is(err, email.ErrMissingTemplateVariable)
}

func (w *Worker) verificationCodeForJob(ctx context.Context, job domain.EmailJob, now time.Time) (string, error) {
	if job.ChallengeID == nil {
		return "", email.PermanentError("invalid_job_data", fmt.Errorf("%s email job missing challenge_id", job.Template))
	}
	if w.store == nil || w.codeSvc == nil {
		return "", email.PermanentError("configuration_error", errors.New("verification code service is not configured"))
	}
	challenge, err := w.store.GetChallengeByID(ctx, *job.ChallengeID)
	if err != nil {
		if errors.Is(err, store.ErrorChallengeNotFound) {
			return "", email.PermanentError("challenge_unavailable", err)
		}
		return "", err
	}
	if challenge.IsConsumed() || challenge.IsExpired(now) {
		return "", email.PermanentError("message_expired", errors.New("email challenge is no longer active"))
	}
	code, err := w.codeSvc.GenerateCode(ctx, challenge, now)
	if errors.Is(err, ErrVerificationCodeSecretNotConfigured) {
		return "", email.PermanentError("configuration_error", err)
	}
	return code, err
}

func (w *Worker) runCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.cleanup(ctx, time.Now().UTC())
		}
	}
}

func (w *Worker) runStaleReaperLoop(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.StaleReaperInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if _, err := w.reapStale(ctx, now.UTC()); err != nil {
				w.logger.ErrorContext(ctx, "failed to recover stale email jobs", "error", err)
			}
		}
	}
}

func (w *Worker) reapStale(ctx context.Context, now time.Time) (int64, error) {
	policy := w.policy.CurrentEmail()
	var total int64
	for {
		recovered, err := w.store.ReapStaleEmailJobs(
			ctx,
			now.Add(-w.cfg.ProcessingStaleAfter),
			now,
			policy.JobMaxAttempts,
			w.cfg.MaintenanceBatchSize,
			w.cfg.RetryBaseDelay,
			w.cfg.RetryMaxDelay,
		)
		if err != nil {
			return total, err
		}
		total += recovered
		if recovered < int64(w.cfg.MaintenanceBatchSize) {
			break
		}
	}
	if total > 0 {
		w.logger.WarnContext(ctx, "recovered stale email jobs", "jobs", total)
	}
	return total, nil
}

func (w *Worker) cleanup(ctx context.Context, now time.Time) {
	policy := w.policy.CurrentEmail()
	if policy.CleanupSentAfter > 0 {
		cutoff := now.Add(-policy.CleanupSentAfter)
		if err := w.store.DeleteSentEmailJobsBefore(ctx, cutoff); err != nil {
			w.logger.ErrorContext(ctx, "failed to cleanup sent email jobs", "error", err)
		}
	}

	if policy.CleanupFailedAfter > 0 {
		cutoff := now.Add(-policy.CleanupFailedAfter)
		if err := w.store.DeleteFailedEmailJobsBefore(ctx, cutoff); err != nil {
			w.logger.ErrorContext(ctx, "failed to cleanup failed email jobs", "error", err)
		}
	}

	if err := w.store.DeleteExpiredChallenges(ctx, now); err != nil {
		w.logger.ErrorContext(ctx, "failed to cleanup expired challenges", "error", err)
	}
}
