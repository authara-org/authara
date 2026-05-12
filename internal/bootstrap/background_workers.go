package bootstrap

import (
	"context"
	"time"
)

func (a *App) StartBackgroundWorkers(ctx context.Context) {
	a.Services.Session.StartCleanupWorker(ctx, a.Logger, 5*time.Minute)

	if a.Config.Challenge.Enabled {
		a.Services.EmailWorker.Run(ctx)
		a.Logger.Info("challenge email workers started",
			"worker_count", a.Config.Email.WorkerCount,
			"provider", a.Config.Email.Provider,
		)
	}
}
