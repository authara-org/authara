package session

import (
	"context"
	"log/slog"
	"time"
)

func (s *Service) StartCleanupWorker(ctx context.Context, logger *slog.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()

		logger.Info("starting session cleanup worker", "interval", interval.String())

		for {
			select {
			case <-ctx.Done():
				logger.Info("stopping session cleanup worker")
				return

			case now := <-ticker.C:
				cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

				if err := s.CleanupExpiredData(cleanupCtx, now); err != nil {
					logger.Error("session cleanup failed", "err", err)
				}

				cancel()
			}
		}
	}()
}
