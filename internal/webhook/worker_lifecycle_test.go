package webhook

import (
	"context"
	"testing"
	"time"
)

func TestWorkerShutdownWaitsForMaintenanceLoop(t *testing.T) {
	worker := NewWorker(nil, nil, nil, WorkerConfig{
		PollInterval:        time.Hour,
		StaleReaperInterval: time.Hour,
		Policy: func() WorkerPolicy {
			return WorkerPolicy{MaintenanceBatchSize: 1}
		},
	})
	worker.Run(context.Background())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-worker.Done():
	default:
		t.Fatal("worker shutdown returned before its goroutines stopped")
	}
}
