package challenge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

func TestEmailWorkerReadsCurrentPolicy(t *testing.T) {
	policy := config.EmailPolicy{JobMaxAttempts: 3, CleanupSentAfter: time.Hour, CleanupFailedAfter: 2 * time.Hour}
	worker := NewWorker(nil, nil, nil, nil, nil, WorkerConfig{
		Policy: config.EmailPolicyReaderFunc(func() config.EmailPolicy { return policy }),
	})
	if got := worker.policy.CurrentEmail().JobMaxAttempts; got != 3 {
		t.Fatalf("initial max attempts = %d", got)
	}
	policy.JobMaxAttempts = 8
	if got := worker.policy.CurrentEmail().JobMaxAttempts; got != 8 {
		t.Fatalf("updated max attempts = %d", got)
	}
}

func TestWorkerDeliveryLogsExcludeRecipientPII(t *testing.T) {
	tests := []struct {
		name        string
		send        senderFunc
		wantMessage string
		wantAttrs   []string
	}{
		{
			name:        "success",
			send:        func(context.Context, string, email.Message) error { return nil },
			wantMessage: "email job sent",
			wantAttrs:   []string{"template=new_sign_in", "attempt=1"},
		},
		{
			name: "retry",
			send: func(_ context.Context, recipient string, _ email.Message) error {
				return email.TransientError("smtp_unavailable", fmt.Errorf("delivery to %s failed", recipient))
			},
			wantMessage: "email job retry scheduled",
			wantAttrs:   []string{"failure_class=transient", "failure_reason=smtp_unavailable"},
		},
		{
			name: "permanent failure",
			send: func(_ context.Context, recipient string, _ email.Message) error {
				return email.PermanentError("smtp_recipient_rejected", fmt.Errorf("recipient %s rejected", recipient))
			},
			wantMessage: "email job permanently failed",
			wantAttrs:   []string{"terminal_reason=smtp_recipient_rejected"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tdb := testutil.OpenTestDB(t)
			now := time.Now().UTC().Add(-time.Minute)
			job := createWorkerTestEmailJob(t, tdb, now, now.Add(72*time.Hour))
			var logs bytes.Buffer
			worker := newQueueTestWorker(tdb, test.send, job.ID)
			worker.logger = slog.New(slog.NewTextHandler(&logs, nil))

			processed, err := worker.RunOnce(context.Background(), now)
			if err != nil || !processed {
				t.Fatalf("RunOnce = (%t, %v), want (true, nil)", processed, err)
			}

			logged := logs.String()
			if strings.Contains(logged, job.ToEmail) {
				t.Fatalf("email worker logged recipient PII: %s", logged)
			}
			for _, want := range append([]string{test.wantMessage, "job_id=" + job.ID.String()}, test.wantAttrs...) {
				if !strings.Contains(logged, want) {
					t.Fatalf("email worker log missing %q: %s", want, logged)
				}
			}
		})
	}
}

func TestEmailRetryDelayUsesCappedEqualJitter(t *testing.T) {
	base := 30 * time.Second
	capDelay := 6 * time.Hour

	if got := emailRetryDelay(1, base, capDelay, 0); got != 15*time.Second {
		t.Fatalf("first minimum retry delay = %s, want 15s", got)
	}
	if got := emailRetryDelay(1, base, capDelay, 1); got >= 30*time.Second || got < 29*time.Second {
		t.Fatalf("first maximum retry delay = %s, want just below 30s", got)
	}
	if got := emailRetryDelay(2, base, capDelay, 0); got != 30*time.Second {
		t.Fatalf("second minimum retry delay = %s, want 30s", got)
	}
	if got := emailRetryDelay(100, base, capDelay, 0.5); got != 4*time.Hour+30*time.Minute {
		t.Fatalf("capped retry delay = %s, want 4h30m", got)
	}
}

func TestWorkerStopsPermanentFailureAfterFirstAttempt(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	now := time.Now().UTC().Add(-time.Minute)
	job := createWorkerTestEmailJob(t, tdb, now, now.Add(72*time.Hour))
	sender := senderFunc(func(context.Context, string, email.Message) error {
		return email.PermanentError("smtp_recipient_rejected", errors.New("smtp: 550 mailbox unavailable"))
	})
	worker := newQueueTestWorker(tdb, sender, job.ID)

	processed, err := worker.RunOnce(context.Background(), now)
	if err != nil || !processed {
		t.Fatalf("RunOnce = (%t, %v), want (true, nil)", processed, err)
	}
	stored, err := tdb.Store.GetEmailJobByID(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != domain.EmailJobStatusFailed || stored.AttemptCount != 1 || stored.FailedAt == nil {
		t.Fatalf("permanent failure state = %+v", stored)
	}
	if stored.TerminalReason == nil || *stored.TerminalReason != "smtp_recipient_rejected" {
		t.Fatalf("terminal reason = %v", stored.TerminalReason)
	}
}

func TestWorkerRetriesTransientFailureAndLaterDelivers(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	now := time.Now().UTC().Add(-time.Minute)
	job := createWorkerTestEmailJob(t, tdb, now, now.Add(72*time.Hour))
	calls := 0
	sender := senderFunc(func(context.Context, string, email.Message) error {
		calls++
		if calls == 1 {
			return email.TransientError("smtp_unavailable", errors.New("smtp: 421 service unavailable"))
		}
		return nil
	})
	worker := newQueueTestWorker(tdb, sender, job.ID)

	processed, err := worker.RunOnce(context.Background(), now)
	if err != nil || !processed {
		t.Fatalf("first RunOnce = (%t, %v), want (true, nil)", processed, err)
	}
	stored, err := tdb.Store.GetEmailJobByID(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	// PostgreSQL stores timestamptz values with microsecond precision.
	wantNext := now.Add(22500 * time.Millisecond).Truncate(time.Microsecond)
	if stored.Status != domain.EmailJobStatusPending || stored.AttemptCount != 1 || !stored.NextAttemptAt.Equal(wantNext) {
		t.Fatalf("retry state = %+v, want next attempt %s", stored, wantNext)
	}

	processed, err = worker.RunOnce(context.Background(), wantNext.Add(-time.Millisecond))
	if err != nil || processed {
		t.Fatalf("early RunOnce = (%t, %v), want (false, nil)", processed, err)
	}
	processed, err = worker.RunOnce(context.Background(), wantNext)
	if err != nil || !processed {
		t.Fatalf("second RunOnce = (%t, %v), want (true, nil)", processed, err)
	}
	stored, err = tdb.Store.GetEmailJobByID(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != domain.EmailJobStatusSent || stored.AttemptCount != 2 || stored.SentAt == nil || stored.ProcessingStartedAt != nil {
		t.Fatalf("delivered state = %+v", stored)
	}
}

func TestWorkerDeliversAfterDayLongProviderOutage(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	createdAt := time.Now().UTC().Add(-time.Minute)
	recoveryAt := createdAt.Add(24 * time.Hour)
	job := createWorkerTestEmailJob(t, tdb, createdAt, createdAt.Add(72*time.Hour))
	currentAttemptAt := createdAt
	sender := senderFunc(func(context.Context, string, email.Message) error {
		if currentAttemptAt.Before(recoveryAt) {
			return email.TransientError("smtp_unavailable", errors.New("smtp provider unavailable"))
		}
		return nil
	})
	worker := newQueueTestWorker(tdb, sender, job.ID)

	for range 100 {
		processed, err := worker.RunOnce(context.Background(), currentAttemptAt)
		if err != nil || !processed {
			t.Fatalf("RunOnce at %s = (%t, %v)", currentAttemptAt, processed, err)
		}
		stored, err := tdb.Store.GetEmailJobByID(context.Background(), job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if stored.Status == domain.EmailJobStatusSent {
			if currentAttemptAt.Before(recoveryAt) {
				t.Fatalf("email delivered before provider recovery at %s", currentAttemptAt)
			}
			return
		}
		if stored.Status != domain.EmailJobStatusPending {
			t.Fatalf("outage job became terminal before recovery: %+v", stored)
		}
		currentAttemptAt = stored.NextAttemptAt
	}
	t.Fatal("email was not delivered after provider recovery")
}

func TestWorkerCreatesTransitionContextAfterSlowSend(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	now := time.Now().UTC().Add(-time.Minute)
	job := createWorkerTestEmailJob(t, tdb, now, now.Add(72*time.Hour))
	sender := senderFunc(func(context.Context, string, email.Message) error {
		time.Sleep(600 * time.Millisecond)
		return nil
	})
	worker := newQueueTestWorker(tdb, sender, job.ID)
	worker.cfg.TransitionTimeout = 500 * time.Millisecond

	processed, err := worker.RunOnce(context.Background(), now)
	if err != nil || !processed {
		t.Fatalf("RunOnce = (%t, %v), want (true, nil)", processed, err)
	}
	stored, err := tdb.Store.GetEmailJobByID(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != domain.EmailJobStatusSent || stored.ProcessingStartedAt != nil {
		t.Fatalf("slow-send state = %+v", stored)
	}
}

func TestWorkerDoesNotDeliverAfterDeliveryDeadline(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	now := time.Now().UTC().Add(-time.Minute)
	job := createWorkerTestEmailJob(t, tdb, now, now)
	calls := 0
	sender := senderFunc(func(context.Context, string, email.Message) error {
		calls++
		return nil
	})
	worker := newQueueTestWorker(tdb, sender, job.ID)

	processed, err := worker.RunOnce(context.Background(), now)
	if err != nil || !processed {
		t.Fatalf("RunOnce = (%t, %v), want (true, nil)", processed, err)
	}
	if calls != 0 {
		t.Fatalf("sender calls = %d, want 0", calls)
	}
	stored, err := tdb.Store.GetEmailJobByID(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != domain.EmailJobStatusFailed || stored.TerminalReason == nil || *stored.TerminalReason != "delivery_deadline_exceeded" {
		t.Fatalf("expired delivery state = %+v", stored)
	}
}

func TestWorkerStopsTransientFailureAtAttemptLimit(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	now := time.Now().UTC().Add(-time.Minute)
	job := createWorkerTestEmailJob(t, tdb, now, now.Add(72*time.Hour))
	if _, err := tdb.Store.DB().ExecContext(context.Background(), `
		UPDATE authara.email_jobs SET attempt_count = 99 WHERE id = $1
	`, job.ID); err != nil {
		t.Fatal(err)
	}
	sender := senderFunc(func(context.Context, string, email.Message) error {
		return email.TransientError("smtp_unavailable", errors.New("smtp: 421 service unavailable"))
	})
	worker := newQueueTestWorker(tdb, sender, job.ID)

	processed, err := worker.RunOnce(context.Background(), now)
	if err != nil || !processed {
		t.Fatalf("RunOnce = (%t, %v), want (true, nil)", processed, err)
	}
	stored, err := tdb.Store.GetEmailJobByID(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != domain.EmailJobStatusFailed || stored.AttemptCount != 100 || stored.TerminalReason == nil || *stored.TerminalReason != "attempts_exhausted" {
		t.Fatalf("attempt exhaustion state = %+v", stored)
	}
}

func TestWorkerReclaimsStaleLeaseAndFencesOldOwner(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	now := time.Now().UTC().Add(-time.Minute)
	job := createWorkerTestEmailJob(t, tdb, now, now.Add(72*time.Hour))
	claimed, err := claimWorkerTestEmailJob(context.Background(), tdb, job.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.ID != job.ID || claimed.AttemptCount != 1 || claimed.ProcessingStartedAt == nil {
		t.Fatalf("claimed job = %+v", claimed)
	}
	oldLease := *claimed.ProcessingStartedAt
	reapAt := now.Add(3 * time.Minute)
	recovered, err := tdb.Store.ReapStaleEmailJobs(context.Background(), reapAt.Add(-2*time.Minute), reapAt, 100, 10, 30*time.Second, 6*time.Hour)
	if err != nil || recovered.Total() != 1 {
		t.Fatalf("ReapStaleEmailJobs = (%+v, %v), want (1, nil)", recovered, err)
	}
	stored, err := tdb.Store.GetEmailJobByID(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != domain.EmailJobStatusPending || stored.AttemptCount != 1 || stored.ProcessingStartedAt != nil {
		t.Fatalf("recovered state = %+v", stored)
	}
	if stored.NextAttemptAt.Before(reapAt.Add(15*time.Second)) || stored.NextAttemptAt.After(reapAt.Add(30*time.Second)) {
		t.Fatalf("recovered next attempt = %s, want equal-jitter window", stored.NextAttemptAt)
	}
	if err := tdb.Store.MarkEmailJobSent(context.Background(), job.ID, oldLease, reapAt); !errors.Is(err, store.ErrorEmailJobLeaseLost) {
		t.Fatalf("old owner completion error = %v, want lease lost", err)
	}

	claimedAgain, err := claimWorkerTestEmailJob(context.Background(), tdb, job.ID, stored.NextAttemptAt)
	if err != nil {
		t.Fatal(err)
	}
	if claimedAgain.ID != job.ID || claimedAgain.AttemptCount != 2 {
		t.Fatalf("reclaimed job = %+v", claimedAgain)
	}
	if err := tdb.Store.MarkEmailJobSent(context.Background(), job.ID, *claimedAgain.ProcessingStartedAt, stored.NextAttemptAt); err != nil {
		t.Fatalf("current owner completion: %v", err)
	}
}

func TestConcurrentStaleReapersRecoverJobOnce(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	now := time.Now().UTC().Add(-time.Minute)
	job := createWorkerTestEmailJob(t, tdb, now, now.Add(72*time.Hour))
	claimed, err := claimWorkerTestEmailJob(context.Background(), tdb, job.ID, now)
	if err != nil || claimed.ID != job.ID {
		t.Fatalf("ClaimNextEmailJob = (%+v, %v)", claimed, err)
	}

	reapAt := now.Add(3 * time.Minute)
	results := make(chan int64, 2)
	errorsCh := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			recovered, reapErr := tdb.Store.ReapStaleEmailJobs(
				context.Background(), reapAt.Add(-2*time.Minute), reapAt, 100, 1, 30*time.Second, 6*time.Hour,
			)
			results <- recovered.Total()
			errorsCh <- reapErr
		}()
	}
	wg.Wait()
	close(results)
	close(errorsCh)

	var total int64
	for recovered := range results {
		total += recovered
	}
	for reapErr := range errorsCh {
		if reapErr != nil {
			t.Fatalf("concurrent reaper: %v", reapErr)
		}
	}
	if total != 1 {
		t.Fatalf("concurrent reapers recovered %d jobs, want 1", total)
	}
	stored, err := tdb.Store.GetEmailJobByID(context.Background(), job.ID)
	if err != nil || stored.Status != domain.EmailJobStatusPending || stored.AttemptCount != 1 {
		t.Fatalf("recovered job = (%+v, %v)", stored, err)
	}
}

func TestStaleReaperTerminatesBoundedJobs(t *testing.T) {
	tests := []struct {
		name           string
		attemptsBefore int
		deadlineOffset time.Duration
		wantReason     string
	}{
		{name: "attempts exhausted", attemptsBefore: 99, deadlineOffset: time.Hour, wantReason: "attempts_exhausted"},
		{name: "deadline exceeded", attemptsBefore: 0, deadlineOffset: time.Minute, wantReason: "delivery_deadline_exceeded"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tdb := testutil.OpenTestDB(t)
			claimAt := time.Now().UTC().Add(-3 * time.Minute)
			job := createWorkerTestEmailJob(t, tdb, claimAt, claimAt.Add(test.deadlineOffset))
			if test.attemptsBefore > 0 {
				if _, err := tdb.Store.DB().ExecContext(context.Background(), `UPDATE authara.email_jobs SET attempt_count = $1 WHERE id = $2`, test.attemptsBefore, job.ID); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := claimWorkerTestEmailJob(context.Background(), tdb, job.ID, claimAt); err != nil {
				t.Fatal(err)
			}
			reapAt := claimAt.Add(3 * time.Minute)
			if recovered, err := tdb.Store.ReapStaleEmailJobs(context.Background(), reapAt.Add(-2*time.Minute), reapAt, 100, 10, 30*time.Second, 6*time.Hour); err != nil || recovered.Total() != 1 {
				t.Fatalf("ReapStaleEmailJobs = (%+v, %v)", recovered, err)
			}
			stored, err := tdb.Store.GetEmailJobByID(context.Background(), job.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.Status != domain.EmailJobStatusFailed || stored.TerminalReason == nil || *stored.TerminalReason != test.wantReason || stored.FailedAt == nil || stored.ProcessingStartedAt != nil {
				t.Fatalf("terminal stale state = %+v, want reason %q", stored, test.wantReason)
			}
		})
	}
}

func TestWorkerShutdownWaitsForInflightSendAndStopsClaims(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	now := time.Now().UTC()
	first := createWorkerTestEmailJob(t, tdb, now.Add(-2*time.Second), now.Add(72*time.Hour))
	second := createWorkerTestEmailJob(t, tdb, now.Add(-time.Second), now.Add(72*time.Hour))
	started := make(chan struct{})
	release := make(chan struct{})
	sender := senderFunc(func(context.Context, string, email.Message) error {
		close(started)
		<-release
		return nil
	})
	worker := newRunningQueueTestWorker(tdb, sender, first.ID, second.ID)
	worker.Run(context.Background())

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("email send did not start")
	}
	shutdownResult := make(chan error, 1)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() { shutdownResult <- worker.Shutdown(shutdownCtx) }()
	select {
	case err := <-shutdownResult:
		t.Fatalf("Shutdown returned before in-flight send completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-shutdownResult; err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	storedFirst, _ := tdb.Store.GetEmailJobByID(context.Background(), first.ID)
	storedSecond, _ := tdb.Store.GetEmailJobByID(context.Background(), second.ID)
	if storedFirst.Status != domain.EmailJobStatusSent || storedSecond.Status != domain.EmailJobStatusPending || storedSecond.AttemptCount != 0 {
		t.Fatalf("shutdown states: first=%+v second=%+v", storedFirst, storedSecond)
	}
}

func TestWorkerForcedShutdownRequeuesCancelledSend(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	now := time.Now().UTC()
	job := createWorkerTestEmailJob(t, tdb, now.Add(-time.Second), now.Add(72*time.Hour))
	started := make(chan struct{})
	cancelled := make(chan struct{})
	releaseAfterCancel := make(chan struct{})
	sender := senderFunc(func(ctx context.Context, _ string, _ email.Message) error {
		close(started)
		<-ctx.Done()
		close(cancelled)
		<-releaseAfterCancel
		return ctx.Err()
	})
	worker := newRunningQueueTestWorker(tdb, sender, job.ID)
	worker.Run(context.Background())
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("email send did not start")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	shutdownResult := make(chan error, 1)
	go func() { shutdownResult <- worker.Shutdown(shutdownCtx) }()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("in-flight send was not cancelled")
	}
	select {
	case err := <-shutdownResult:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Shutdown error = %v, want deadline exceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not respect its deadline")
	}
	close(releaseAfterCancel)
	select {
	case <-worker.Done():
	case <-time.After(time.Second):
		t.Fatal("email worker did not settle after the blocked sender returned")
	}

	stored, err := tdb.Store.GetEmailJobByID(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != domain.EmailJobStatusPending || stored.AttemptCount != 1 || stored.ProcessingStartedAt != nil {
		t.Fatalf("cancelled send state = %+v", stored)
	}
}

func TestWorkerSuppliesCodeToEveryCodeTemplate(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
		codeService := NewVerificationCodeService(
			tdb.Store,
			10*time.Minute,
			[]byte("01234567890123456789012345678901"),
		)
		cases := []struct {
			name     string
			purpose  domain.ChallengePurpose
			template domain.EmailTemplate
		}{
			{name: "signup", purpose: domain.ChallengePurposeSignup, template: domain.EmailTemplateSignupCode},
			{name: "password reset", purpose: domain.ChallengePurposePasswordReset, template: domain.EmailTemplatePasswordResetCode},
			{name: "email change", purpose: domain.ChallengePurposeEmailChange, template: domain.EmailTemplateEmailChangeCode},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				challenge, err := tdb.Store.CreateChallenge(ctx, domain.Challenge{
					Purpose:     tc.purpose,
					Email:       "code-template@example.test",
					ExpiresAt:   now.Add(30 * time.Minute),
					MaxAttempts: 5,
				})
				if err != nil {
					t.Fatalf("create challenge: %v", err)
				}

				var renderedTemplate domain.EmailTemplate
				var renderedData email.TemplateData
				renderer := templateRendererFunc(func(_ context.Context, key domain.EmailTemplate, data email.TemplateData) (email.Message, error) {
					renderedTemplate = key
					renderedData = data
					return email.Message{Subject: "subject", Text: "text"}, nil
				})
				sender := &recordingEmailSender{}
				worker := NewWorker(tdb.Store, codeService, renderer, sender, nil, WorkerConfig{})
				if err := worker.processJob(ctx, domain.EmailJob{
					ChallengeID: &challenge.ID,
					ToEmail:     challenge.Email,
					Template:    tc.template,
				}, now); err != nil {
					t.Fatalf("process email job: %v", err)
				}

				if renderedTemplate != tc.template {
					t.Fatalf("rendered template = %q, want %q", renderedTemplate, tc.template)
				}
				code := renderedData[email.TemplateVariableCode]
				if len(code) != 6 || !onlyDecimalDigits(code) {
					t.Fatalf("rendered code = %q, want six digits", code)
				}
				if len(renderedData) != 1 {
					t.Fatalf("rendered template data = %#v", renderedData)
				}
				if len(sender.messages) != 1 {
					t.Fatalf("sent messages = %d, want 1", len(sender.messages))
				}
			})
		}
	})
}

func TestChallengeEmailRetryMakesNewestGeneratedCodeAuthoritative(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
		challengeRow, err := tdb.Store.CreateChallenge(ctx, domain.Challenge{
			Purpose:     domain.ChallengePurposeSignup,
			Email:       "code-retry@example.test",
			ExpiresAt:   now.Add(30 * time.Minute),
			MaxAttempts: 5,
		})
		if err != nil {
			t.Fatal(err)
		}
		codeService := NewVerificationCodeService(
			tdb.Store,
			10*time.Minute,
			[]byte("01234567890123456789012345678901"),
		)
		var deliveredCodes []string
		renderer := templateRendererFunc(func(_ context.Context, _ domain.EmailTemplate, data email.TemplateData) (email.Message, error) {
			deliveredCodes = append(deliveredCodes, data[email.TemplateVariableCode])
			return email.Message{Subject: "code", Text: "code"}, nil
		})
		worker := NewWorker(tdb.Store, codeService, renderer, &recordingEmailSender{}, nil, WorkerConfig{})
		job := domain.EmailJob{
			ChallengeID: &challengeRow.ID,
			ToEmail:     challengeRow.Email,
			Template:    domain.EmailTemplateSignupCode,
		}

		if err := worker.processJob(ctx, job, now); err != nil {
			t.Fatal(err)
		}
		firstCode := deliveredCodes[0]
		if err := codeService.VerifyCode(ctx, challengeRow.ID, firstCode, now); err != nil {
			t.Fatalf("first code should initially verify: %v", err)
		}

		latestCode := firstCode
		for range 10 {
			if err := worker.processJob(ctx, job, now.Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			latestCode = deliveredCodes[len(deliveredCodes)-1]
			if latestCode != firstCode {
				break
			}
		}
		if latestCode == firstCode {
			t.Fatal("generated codes unexpectedly remained identical")
		}
		if err := codeService.VerifyCode(ctx, challengeRow.ID, firstCode, now.Add(time.Second)); !errors.Is(err, ErrInvalidVerificationCode) {
			t.Fatalf("first code verification = %v, want ErrInvalidVerificationCode", err)
		}
		if err := codeService.VerifyCode(ctx, challengeRow.ID, latestCode, now.Add(time.Second)); err != nil {
			t.Fatalf("latest code should verify: %v", err)
		}
	})
}

func onlyDecimalDigits(value string) bool {
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func TestWorkerRendersInvitationAtDeliveryTime(t *testing.T) {
	templateData := email.TemplateData{
		email.TemplateVariableOrganizationName: "Acme",
		email.TemplateVariableInviteURL:        "https://authara.example/auth/invitations/accept?token=abc",
		email.TemplateVariableInvitationCode:   "invite-123",
		email.TemplateVariableRole:             "member",
		email.TemplateVariableExpiresAt:        "2026-09-07T12:00:00Z",
	}
	rawTemplateData, err := json.Marshal(templateData)
	if err != nil {
		t.Fatalf("marshal template data: %v", err)
	}

	templateStore := &currentTemplateStore{}
	renderer := email.NewTemplateService(templateStore)
	sender := &recordingEmailSender{}
	worker := NewWorker(nil, nil, renderer, sender, nil, WorkerConfig{})
	job := domain.EmailJob{
		ToEmail:      "member@example.com",
		Template:     domain.EmailTemplateOrganizationInvite,
		TemplateData: rawTemplateData,
	}

	if err := worker.processJob(context.Background(), job, time.Now()); err != nil {
		t.Fatalf("process first job: %v", err)
	}
	templateStore.override = &domain.EmailTemplateOverride{
		Template:        domain.EmailTemplateOrganizationInvite,
		SubjectTemplate: "Customized invitation for {{organization_name}}",
		TextTemplate:    "Join at {{invite_url}} with {{invitation_code}} as {{role}} before {{expires_at}}.",
		HTMLTemplate:    "<p>Join at {{invite_url}} with {{invitation_code}} as {{role}} before {{expires_at}}.</p>",
		Revision:        1,
	}
	if err := worker.processJob(context.Background(), job, time.Now()); err != nil {
		t.Fatalf("process second job: %v", err)
	}

	if len(sender.messages) != 2 {
		t.Fatalf("sent messages = %d, want 2", len(sender.messages))
	}
	if sender.messages[0].Subject != "You're invited to Acme" || sender.messages[1].Subject != "Customized invitation for Acme" {
		t.Fatalf("sent subjects = %q, %q", sender.messages[0].Subject, sender.messages[1].Subject)
	}
	if sender.messages[1].Text != "Join at https://authara.example/auth/invitations/accept?token=abc with invite-123 as member before 2026-09-07T12:00:00Z." {
		t.Fatalf("customized text = %q", sender.messages[1].Text)
	}
}

func TestWorkerRendersNotificationTemplateData(t *testing.T) {
	templateData := email.TemplateData{
		email.TemplateVariableIPAddress:  "203.0.113.42",
		email.TemplateVariableUserAgent:  "Test Browser",
		email.TemplateVariableOccurredAt: "2026-09-09T18:30:00Z",
	}
	rawTemplateData, err := json.Marshal(templateData)
	if err != nil {
		t.Fatalf("marshal template data: %v", err)
	}

	var renderedData email.TemplateData
	renderer := templateRendererFunc(func(_ context.Context, key domain.EmailTemplate, data email.TemplateData) (email.Message, error) {
		if key != domain.EmailTemplateNewSignIn {
			t.Fatalf("rendered template = %q, want %q", key, domain.EmailTemplateNewSignIn)
		}
		renderedData = data
		return email.Message{Subject: "subject", Text: "text"}, nil
	})
	sender := &recordingEmailSender{}
	worker := NewWorker(nil, nil, renderer, sender, nil, WorkerConfig{})
	if err := worker.processJob(context.Background(), domain.EmailJob{
		ToEmail:      "user@example.com",
		Template:     domain.EmailTemplateNewSignIn,
		TemplateData: rawTemplateData,
	}, time.Now()); err != nil {
		t.Fatalf("process notification job: %v", err)
	}
	if renderedData[email.TemplateVariableIPAddress] != "203.0.113.42" ||
		renderedData[email.TemplateVariableUserAgent] != "Test Browser" ||
		renderedData[email.TemplateVariableOccurredAt] != "2026-09-09T18:30:00Z" {
		t.Fatalf("rendered template data = %#v", renderedData)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("sent messages = %d, want 1", len(sender.messages))
	}
}

func TestWorkerDoesNotSendWhenTemplateRenderingFails(t *testing.T) {
	renderErr := errors.New("render failed")
	renderer := templateRendererFunc(func(context.Context, domain.EmailTemplate, email.TemplateData) (email.Message, error) {
		return email.Message{}, renderErr
	})
	sender := &recordingEmailSender{}
	worker := NewWorker(nil, nil, renderer, sender, nil, WorkerConfig{})
	templateData, err := json.Marshal(email.TemplateData{
		email.TemplateVariableOrganizationName: "Acme",
		email.TemplateVariableInviteURL:        "https://authara.example/invite",
		email.TemplateVariableInvitationCode:   "invite-123",
		email.TemplateVariableRole:             "member",
		email.TemplateVariableExpiresAt:        "2026-09-07T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("marshal template data: %v", err)
	}

	err = worker.processJob(context.Background(), domain.EmailJob{
		ToEmail:      "member@example.com",
		Template:     domain.EmailTemplateOrganizationInvite,
		TemplateData: templateData,
	}, time.Now())
	if !errors.Is(err, renderErr) {
		t.Fatalf("process job error = %v, want render error", err)
	}
	if len(sender.messages) != 0 {
		t.Fatalf("sent messages = %d, want 0", len(sender.messages))
	}
}

func TestWorkerClassifiesMissingDeliveryConfigurationAsPermanent(t *testing.T) {
	worker := NewWorker(nil, nil, nil, nil, nil, WorkerConfig{})
	err := worker.processJob(context.Background(), domain.EmailJob{
		ToEmail:  "user@example.com",
		Template: domain.EmailTemplateNewSignIn,
	}, time.Now())
	class, reason := email.ClassifyFailure(err)
	if class != email.FailurePermanent || reason != "configuration_error" {
		t.Fatalf("configuration failure = (%q, %q, %v)", class, reason, err)
	}
}

type templateRendererFunc func(context.Context, domain.EmailTemplate, email.TemplateData) (email.Message, error)

func (f templateRendererFunc) Render(ctx context.Context, key domain.EmailTemplate, data email.TemplateData) (email.Message, error) {
	return f(ctx, key, data)
}

type currentTemplateStore struct {
	email.TemplateOverrideStore
	override *domain.EmailTemplateOverride
}

func (s *currentTemplateStore) GetEmailTemplateOverride(_ context.Context, key domain.EmailTemplate) (domain.EmailTemplateOverride, error) {
	if s.override == nil || s.override.Template != key {
		return domain.EmailTemplateOverride{}, store.ErrEmailTemplateOverrideNotFound
	}
	return *s.override, nil
}

type recordingEmailSender struct {
	messages []email.Message
}

func (s *recordingEmailSender) Send(_ context.Context, _ string, message email.Message) error {
	s.messages = append(s.messages, message)
	return nil
}

type senderFunc func(context.Context, string, email.Message) error

func (f senderFunc) Send(ctx context.Context, to string, message email.Message) error {
	return f(ctx, to, message)
}

func createWorkerTestEmailJob(
	t *testing.T,
	tdb *testutil.TestDB,
	nextAttemptAt time.Time,
	deliveryDeadlineAt time.Time,
) domain.EmailJob {
	t.Helper()
	job, err := tdb.Store.CreateEmailJob(context.Background(), domain.EmailJob{
		ToEmail:            fmt.Sprintf("worker-%d@example.test", time.Now().UnixNano()),
		Template:           domain.EmailTemplateNewSignIn,
		Status:             domain.EmailJobStatusPending,
		NextAttemptAt:      nextAttemptAt,
		DeliveryDeadlineAt: deliveryDeadlineAt,
	})
	if err != nil {
		t.Fatalf("create email job: %v", err)
	}
	t.Cleanup(func() {
		if _, err := tdb.Store.DB().ExecContext(context.Background(), `DELETE FROM email_jobs WHERE id = $1`, job.ID); err != nil {
			t.Errorf("delete email job %s: %v", job.ID, err)
		}
	})
	return job
}

func newQueueTestWorker(tdb *testutil.TestDB, sender email.Sender, jobIDs ...uuid.UUID) *Worker {
	renderer := templateRendererFunc(func(context.Context, domain.EmailTemplate, email.TemplateData) (email.Message, error) {
		return email.Message{Subject: "test", Text: "test"}, nil
	})
	worker := NewWorker(tdb.Store, nil, renderer, sender, nil, WorkerConfig{
		SendTimeout:    time.Second,
		JobMaxAttempts: 100,
		RetryBaseDelay: 30 * time.Second,
		RetryMaxDelay:  6 * time.Hour,
		RandomFloat64:  func() float64 { return 0.5 },
	})
	worker.claimNext = func(ctx context.Context, now time.Time) (domain.EmailJob, error) {
		for _, jobID := range jobIDs {
			job, err := claimWorkerTestEmailJob(ctx, tdb, jobID, now)
			if err == nil {
				return job, nil
			}
			if !errors.Is(err, store.ErrorEmailJobNotFound) {
				return domain.EmailJob{}, err
			}
		}
		return domain.EmailJob{}, store.ErrorEmailJobNotFound
	}
	return worker
}

func newRunningQueueTestWorker(tdb *testutil.TestDB, sender email.Sender, jobIDs ...uuid.UUID) *Worker {
	worker := newQueueTestWorker(tdb, sender, jobIDs...)
	worker.cfg.WorkerCount = 1
	worker.cfg.PollInterval = time.Millisecond
	worker.cfg.StaleReaperInterval = time.Hour
	return worker
}

func claimWorkerTestEmailJob(ctx context.Context, tdb *testutil.TestDB, jobID uuid.UUID, now time.Time) (domain.EmailJob, error) {
	result, err := tdb.Store.DB().ExecContext(ctx, `
		UPDATE authara.email_jobs
		SET status = $1,
		    attempt_count = attempt_count + 1,
		    processing_started_at = $2
		WHERE id = $3
		  AND status = $4
		  AND next_attempt_at <= $2
	`, domain.EmailJobStatusProcessing, now, jobID, domain.EmailJobStatusPending)
	if err != nil {
		return domain.EmailJob{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.EmailJob{}, err
	}
	if affected != 1 {
		return domain.EmailJob{}, store.ErrorEmailJobNotFound
	}
	return tdb.Store.GetEmailJobByID(ctx, jobID)
}
