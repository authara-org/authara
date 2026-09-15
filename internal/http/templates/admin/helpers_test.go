package admin

import (
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
)

func TestEmailQueueAgeStopsAtTerminalTransition(t *testing.T) {
	createdAt := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	failedAt := createdAt.Add(45 * time.Minute)
	job := domain.EmailJob{CreatedAt: createdAt, FailedAt: &failedAt}

	if got := emailQueueAge(job, createdAt.Add(24*time.Hour)); got != 45*time.Minute {
		t.Fatalf("queue age = %s, want 45m", got)
	}
}

func TestEmailQueueAgeContinuesForQueuedJob(t *testing.T) {
	createdAt := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	job := domain.EmailJob{CreatedAt: createdAt, Status: domain.EmailJobStatusPending}

	if got := emailQueueAge(job, createdAt.Add(2*time.Hour)); got != 2*time.Hour {
		t.Fatalf("queue age = %s, want 2h", got)
	}
}
