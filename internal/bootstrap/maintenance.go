package bootstrap

import (
	"context"
	"time"

	"github.com/authara-org/authara/internal/maintenance"
)

const cleanupBatchSize = 1000

func newCleanupCoordinator(app *App) (*maintenance.Coordinator, error) {
	cfg := app.Config.Startup()
	jobs := []maintenance.Job{
		{
			Name:     "sessions_expired",
			Interval: cfg.Session.CleanupInterval,
			RunBatch: func(ctx context.Context, now time.Time) (int64, bool, error) {
				rows, err := app.Store.DeleteExpiredSessions(ctx, now, cleanupBatchSize)
				return rows, rows == int64(cleanupBatchSize), err
			},
		},
		{
			Name:     "sessions_revoked",
			Interval: cfg.Session.CleanupInterval,
			RunBatch: func(ctx context.Context, _ time.Time) (int64, bool, error) {
				rows, err := app.Store.DeleteRevokedSessions(ctx, cleanupBatchSize)
				return rows, rows == int64(cleanupBatchSize), err
			},
		},
		{
			Name:     "refresh_tokens",
			Interval: cfg.Session.CleanupInterval,
			RunBatch: func(ctx context.Context, now time.Time) (int64, bool, error) {
				rows, err := app.Store.DeleteExpiredRefreshTokens(ctx, now, cleanupBatchSize)
				return rows, rows == int64(cleanupBatchSize), err
			},
		},
		{
			Name:     "webauthn_challenges",
			Interval: cfg.Session.CleanupInterval,
			RunBatch: func(ctx context.Context, now time.Time) (int64, bool, error) {
				rows, err := app.Store.DeleteExpiredWebAuthnChallenges(ctx, now, cleanupBatchSize)
				return rows, rows == int64(cleanupBatchSize), err
			},
		},
		{
			Name:     "email_sent",
			Interval: cfg.Email.CleanupInterval,
			RunBatch: func(ctx context.Context, now time.Time) (int64, bool, error) {
				policy := app.Config.CurrentEmail()
				rows, err := app.Store.DeleteSentEmailJobsBefore(
					ctx, now.Add(-policy.CleanupSentAfter), cfg.Email.MaintenanceBatchSize,
				)
				return rows, rows == int64(cfg.Email.MaintenanceBatchSize), err
			},
		},
		{
			Name:     "email_failed",
			Interval: cfg.Email.CleanupInterval,
			RunBatch: func(ctx context.Context, now time.Time) (int64, bool, error) {
				policy := app.Config.CurrentEmail()
				rows, err := app.Store.DeleteFailedEmailJobsBefore(
					ctx, now.Add(-policy.CleanupFailedAfter), cfg.Email.MaintenanceBatchSize,
				)
				return rows, rows == int64(cfg.Email.MaintenanceBatchSize), err
			},
		},
		{
			Name:     "challenges",
			Interval: cfg.Email.CleanupInterval,
			RunBatch: func(ctx context.Context, now time.Time) (int64, bool, error) {
				rows, err := app.Store.DeleteExpiredChallenges(ctx, now, cfg.Email.MaintenanceBatchSize)
				return rows, rows == int64(cfg.Email.MaintenanceBatchSize), err
			},
		},
		{
			Name:     "admin_audit",
			Interval: cfg.Admin.AuditCleanupInterval,
			RunBatch: func(ctx context.Context, now time.Time) (int64, bool, error) {
				return app.Services.Admin.CleanupExpiredAuditEventsBatch(ctx, now, cleanupBatchSize)
			},
		},
		{
			Name:     "security_events",
			Interval: cfg.SecurityEvents.CleanupInterval,
			RunBatch: func(ctx context.Context, now time.Time) (int64, bool, error) {
				return app.Services.SecurityEvents.CleanupExpiredBatch(ctx, now, cleanupBatchSize)
			},
		},
	}
	if app.Services.WebhookWorker != nil {
		jobs = append(jobs, maintenance.Job{
			Name:     "webhook",
			Interval: cfg.Webhook.CleanupInterval,
			RunBatch: app.Services.WebhookWorker.CleanupExpiredEventsBatch,
		})
	}

	return maintenance.New(app.Store, app.Logger, app.Observability, jobs, maintenance.Config{})
}
