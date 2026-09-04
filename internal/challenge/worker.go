package challenge

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/store"
)

type WorkerConfig struct {
	WorkerCount        int
	PollInterval       time.Duration
	JobMaxAttempts     int
	CleanupSentAfter   time.Duration
	CleanupFailedAfter time.Duration
	CleanupInterval    time.Duration
	SendTimeout        time.Duration
	Metrics            WorkerMetrics
}

type WorkerMetrics interface {
	ObserveBackgroundJob(worker, outcome string, duration time.Duration)
}

type Worker struct {
	store   *store.Store
	codeSvc *VerificationCodeService
	sender  email.Sender
	logger  *slog.Logger
	metrics WorkerMetrics
	cfg     WorkerConfig
}

func NewWorker(
	store *store.Store,
	codeSvc *VerificationCodeService,
	sender email.Sender,
	logger *slog.Logger,
	cfg WorkerConfig,
) *Worker {
	if logger == nil {
		logger = slog.Default()
	}

	return &Worker{
		store:   store,
		codeSvc: codeSvc,
		sender:  sender,
		logger:  logger,
		metrics: cfg.Metrics,
		cfg:     cfg,
	}
}

func (w *Worker) Run(ctx context.Context) {
	for i := range w.cfg.WorkerCount {
		go w.runWorker(ctx, i+1)
	}

	if w.cfg.CleanupInterval > 0 {
		go w.runCleanupLoop(ctx)
	}
}

func (w *Worker) runWorker(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		processed, err := w.RunOnce(ctx, time.Now().UTC())
		if err != nil {
			w.logger.ErrorContext(ctx, "email worker iteration failed",
				"worker_id", workerID,
				"error", err,
			)

			select {
			case <-ctx.Done():
				return
			case <-time.After(w.cfg.PollInterval):
			}
			continue
		}

		if processed {
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
	job, err := w.store.ClaimNextEmailJob(ctx, now)
	if err != nil {
		if errors.Is(err, store.ErrorEmailJobNotFound) {
			return false, nil
		}
		return false, err
	}
	started := time.Now()

	sendCtx, cancel := context.WithTimeout(ctx, w.cfg.SendTimeout)
	defer cancel()

	if err := w.processJob(sendCtx, job, now); err != nil {
		if job.AttemptCount+1 >= w.cfg.JobMaxAttempts {
			if markErr := w.store.MarkEmailJobFailed(ctx, job.ID, err.Error()); markErr != nil {
				w.observeJob("error", started)
			} else {
				w.observeJob("failed", started)
			}
			return true, err
		}

		if requeueErr := w.store.RequeueEmailJob(ctx, job.ID, err.Error(), now.Add(30*time.Second)); requeueErr != nil {
			w.observeJob("error", started)
		} else {
			w.observeJob("retried", started)
		}
		return true, err
	}

	if err := w.store.MarkEmailJobSent(ctx, job.ID, now); err != nil {
		w.observeJob("error", started)
		return true, err
	}

	if job.ChallengeID != nil {
		_ = w.store.SetChallengeLastSentAt(ctx, *job.ChallengeID, now)
	}

	w.logger.InfoContext(ctx, "email job sent",
		"job_id", job.ID,
		"template", job.Template,
		"to_email", job.ToEmail,
	)
	w.observeJob("succeeded", started)

	return true, nil
}

func (w *Worker) observeJob(outcome string, started time.Time) {
	if w.metrics != nil {
		w.metrics.ObserveBackgroundJob("email", outcome, time.Since(started))
	}
}

func (w *Worker) processJob(ctx context.Context, job domain.EmailJob, now time.Time) error {
	var msg email.Message
	var err error

	switch job.Template {
	case domain.EmailTemplateSignupCode:
		if job.ChallengeID == nil {
			return errors.New("signup_code email job missing challenge_id")
		}

		challenge, err := w.store.GetChallengeByID(ctx, *job.ChallengeID)
		if err != nil {
			return err
		}

		code, err := w.codeSvc.GenerateCode(ctx, challenge, now)
		if err != nil {
			return err
		}

		msg, err = email.BuildSignupCodeMessage(code)
		if err != nil {
			return err
		}

	case domain.EmailTemplatePasswordResetCode:
		if job.ChallengeID == nil {
			return errors.New("password_reset_code email job missing challenge_id")
		}

		challenge, err := w.store.GetChallengeByID(ctx, *job.ChallengeID)
		if err != nil {
			return err
		}

		code, err := w.codeSvc.GenerateCode(ctx, challenge, now)
		if err != nil {
			return err
		}

		msg, err = email.BuildPasswordResetCodeMessage(code)
		if err != nil {
			return err
		}

	case domain.EmailTemplateEmailChangeCode:
		if job.ChallengeID == nil {
			return errors.New("email_change_code email job missing challenge_id")
		}

		challenge, err := w.store.GetChallengeByID(ctx, *job.ChallengeID)
		if err != nil {
			return err
		}

		code, err := w.codeSvc.GenerateCode(ctx, challenge, now)
		if err != nil {
			return err
		}

		msg, err = email.BuildEmailChangeCodeMessage(code)
		if err != nil {
			return err
		}

	case domain.EmailTemplateOrganizationInvite:
		var payload email.OrganizationInvitationPayload
		if len(job.TemplateData) > 0 {
			if err := json.Unmarshal(job.TemplateData, &payload); err != nil {
				return err
			}
		}

		msg, err = email.BuildOrganizationInvitationMessage(payload)
		if err != nil {
			return err
		}

	default:
		return errors.New("unsupported email template")
	}

	return w.sender.Send(ctx, job.ToEmail, msg)
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

func (w *Worker) cleanup(ctx context.Context, now time.Time) {
	if w.cfg.CleanupSentAfter > 0 {
		cutoff := now.Add(-w.cfg.CleanupSentAfter)
		if err := w.store.DeleteSentEmailJobsBefore(ctx, cutoff); err != nil {
			w.logger.ErrorContext(ctx, "failed to cleanup sent email jobs", "error", err)
		}
	}

	if w.cfg.CleanupFailedAfter > 0 {
		cutoff := now.Add(-w.cfg.CleanupFailedAfter)
		if err := w.store.DeleteFailedEmailJobsBefore(ctx, cutoff); err != nil {
			w.logger.ErrorContext(ctx, "failed to cleanup failed email jobs", "error", err)
		}
	}

	if err := w.store.DeleteExpiredChallenges(ctx, now); err != nil {
		w.logger.ErrorContext(ctx, "failed to cleanup expired challenges", "error", err)
	}
}
