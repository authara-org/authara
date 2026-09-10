package email

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
)

func TestEnqueueCreatesRenderablePendingJob(t *testing.T) {
	store := &recordingJobCreator{}
	now := time.Date(2026, 9, 9, 18, 30, 0, 0, time.FixedZone("test", 2*60*60))
	data := TemplateData{
		TemplateVariableAuthMethod: "google",
		TemplateVariableOccurredAt: OccurredAt(now),
	}
	if err := Enqueue(
		context.Background(),
		store,
		" user@example.com ",
		domain.EmailTemplateAuthMethodAdded,
		data,
		now,
	); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	if len(store.jobs) != 1 {
		t.Fatalf("created jobs = %d, want 1", len(store.jobs))
	}
	job := store.jobs[0]
	if job.ToEmail != "user@example.com" || job.Template != domain.EmailTemplateAuthMethodAdded {
		t.Fatalf("unexpected job identity: %+v", job)
	}
	if job.Status != domain.EmailJobStatusPending || !job.NextAttemptAt.Equal(now) {
		t.Fatalf("unexpected job schedule: %+v", job)
	}
	var got TemplateData
	if err := json.Unmarshal(job.TemplateData, &got); err != nil {
		t.Fatalf("decode template data: %v", err)
	}
	if got[TemplateVariableAuthMethod] != "google" || got[TemplateVariableOccurredAt] != "2026-09-09T16:30:00Z" {
		t.Fatalf("unexpected template data: %#v", got)
	}
	if _, err := RenderBuiltInTemplate(job.Template, got); err != nil {
		t.Fatalf("queued data does not render: %v", err)
	}
}

func TestEnqueueRejectsIncompleteDataBeforeCreatingJob(t *testing.T) {
	store := &recordingJobCreator{}
	err := Enqueue(
		context.Background(),
		store,
		"user@example.com",
		domain.EmailTemplateAuthMethodAdded,
		TemplateData{TemplateVariableAuthMethod: "google"},
		time.Now(),
	)
	if !errors.Is(err, ErrMissingTemplateVariable) {
		t.Fatalf("Enqueue error = %v, want ErrMissingTemplateVariable", err)
	}
	if len(store.jobs) != 0 {
		t.Fatalf("created jobs = %d, want 0", len(store.jobs))
	}
}

type recordingJobCreator struct {
	jobs []domain.EmailJob
}

func (s *recordingJobCreator) CreateEmailJob(_ context.Context, job domain.EmailJob) (domain.EmailJob, error) {
	s.jobs = append(s.jobs, job)
	return job, nil
}
