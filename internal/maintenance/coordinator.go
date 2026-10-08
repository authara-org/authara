package maintenance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

const (
	defaultLeaseDuration        = 30 * time.Second
	defaultRenewInterval        = 10 * time.Second
	defaultRetryInterval        = 5 * time.Second
	defaultContinuationInterval = 5 * time.Second
	defaultPassTimeout          = 10 * time.Second
	defaultMaxBatches           = 10
)

type LeaseStore interface {
	TryAcquireMaintenanceLease(context.Context, string, uuid.UUID, time.Duration) (store.MaintenanceLease, bool, error)
	RenewMaintenanceLease(context.Context, store.MaintenanceLease, time.Duration) (store.MaintenanceLease, bool, error)
	ReleaseMaintenanceLease(context.Context, store.MaintenanceLease) (bool, error)
	RunMaintenanceBatch(context.Context, store.MaintenanceLease, func(context.Context) (int64, bool, error)) (int64, bool, bool, error)
}

type Metrics interface {
	ObserveMaintenanceLease(outcome string)
	SetMaintenanceLeader(leader bool)
	ObserveMaintenanceRun(job, outcome string, duration time.Duration, rows int64)
}

type BatchFunc func(context.Context, time.Time) (rows int64, more bool, err error)

type Job struct {
	Name     string
	Interval time.Duration
	RunBatch BatchFunc
}

type Config struct {
	LeaseDuration        time.Duration
	RenewInterval        time.Duration
	RetryInterval        time.Duration
	ContinuationInterval time.Duration
	PassTimeout          time.Duration
	MaxBatches           int
}

type Coordinator struct {
	store   LeaseStore
	logger  *slog.Logger
	metrics Metrics
	ownerID uuid.UUID
	jobs    []Job
	cfg     Config

	mu          sync.Mutex
	started     bool
	cancel      context.CancelFunc
	done        chan struct{}
	shutdownCtx context.Context
}

func New(store LeaseStore, logger *slog.Logger, metrics Metrics, jobs []Job, cfg Config) (*Coordinator, error) {
	if store == nil {
		return nil, errors.New("maintenance lease store is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.LeaseDuration <= 0 {
		cfg.LeaseDuration = defaultLeaseDuration
	}
	if cfg.RenewInterval <= 0 {
		cfg.RenewInterval = defaultRenewInterval
	}
	if cfg.RetryInterval <= 0 {
		cfg.RetryInterval = defaultRetryInterval
	}
	if cfg.ContinuationInterval <= 0 {
		cfg.ContinuationInterval = defaultContinuationInterval
	}
	if cfg.PassTimeout <= 0 {
		cfg.PassTimeout = defaultPassTimeout
	}
	if cfg.MaxBatches <= 0 {
		cfg.MaxBatches = defaultMaxBatches
	}
	if cfg.RenewInterval >= cfg.LeaseDuration {
		return nil, errors.New("maintenance lease renewal interval must be shorter than its duration")
	}
	if cfg.PassTimeout >= cfg.LeaseDuration {
		return nil, errors.New("maintenance pass timeout must be shorter than the lease duration")
	}
	for _, job := range jobs {
		if job.Name == "" {
			return nil, errors.New("maintenance job name is required")
		}
		if job.Interval <= 0 {
			return nil, fmt.Errorf("maintenance job %q interval must be positive", job.Name)
		}
		if job.RunBatch == nil {
			return nil, fmt.Errorf("maintenance job %q batch function is required", job.Name)
		}
	}

	return &Coordinator{
		store: store, logger: logger, metrics: metrics, ownerID: uuid.New(),
		jobs: append([]Job(nil), jobs...), cfg: cfg, done: make(chan struct{}),
	}, nil
}

func (c *Coordinator) Run(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return
	}
	workerCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.started = true
	go func() {
		defer close(c.done)
		c.run(workerCtx)
	}()
}

func (c *Coordinator) Shutdown(ctx context.Context) error {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return nil
	}
	c.shutdownCtx = ctx
	c.cancel()
	done := c.done
	c.mu.Unlock()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Coordinator) Done() <-chan struct{} {
	return c.done
}

func (c *Coordinator) run(ctx context.Context) {
	for {
		lease, acquired, err := c.store.TryAcquireMaintenanceLease(
			ctx, store.CleanupLeaseName, c.ownerID, c.cfg.LeaseDuration,
		)
		if err != nil {
			if ctx.Err() == nil {
				c.observeLease("failed")
				c.logger.ErrorContext(ctx, "cleanup lease acquisition failed", "error", err)
			}
		} else if !acquired {
			c.observeLease("skipped")
		} else {
			c.observeLease("acquired")
			c.setLeader(true)
			c.logger.InfoContext(ctx, "cleanup leadership acquired", "generation", lease.Generation)
			c.runAsLeader(ctx, lease)
			c.setLeader(false)
		}

		if !wait(ctx, c.cfg.RetryInterval) {
			return
		}
	}
}

func (c *Coordinator) runAsLeader(parent context.Context, lease store.MaintenanceLease) {
	ctx, cancel := context.WithCancel(parent)
	renewed := make(chan struct{})
	go func() {
		defer close(renewed)
		c.renewLease(ctx, cancel, lease)
	}()

	c.runScheduler(ctx, lease)
	cancel()
	<-renewed

	releaseParent := context.Background()
	c.mu.Lock()
	if c.shutdownCtx != nil {
		releaseParent = c.shutdownCtx
	}
	c.mu.Unlock()
	releaseCtx, releaseCancel := context.WithTimeout(releaseParent, 2*time.Second)
	defer releaseCancel()
	released, err := c.store.ReleaseMaintenanceLease(releaseCtx, lease)
	if err != nil {
		c.observeLease("failed")
		c.logger.Error("cleanup lease release failed", "error", err)
		return
	}
	if released {
		c.observeLease("released")
		c.logger.Info("cleanup leadership released", "generation", lease.Generation)
	}
}

func (c *Coordinator) renewLease(ctx context.Context, cancel context.CancelFunc, lease store.MaintenanceLease) {
	ticker := time.NewTicker(c.cfg.RenewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, renewed, err := c.store.RenewMaintenanceLease(ctx, lease, c.cfg.LeaseDuration)
			if err != nil {
				c.observeLease("failed")
				c.logger.ErrorContext(ctx, "cleanup lease renewal failed", "error", err)
				cancel()
				return
			}
			if !renewed {
				c.observeLease("lost")
				c.logger.WarnContext(ctx, "cleanup leadership lost", "generation", lease.Generation)
				cancel()
				return
			}
		}
	}
}

type scheduledJob struct {
	Job
	next time.Time
}

type jobRunOutcome string

const (
	jobRunCompleted  jobRunOutcome = "completed"
	jobRunIncomplete jobRunOutcome = "incomplete"
	jobRunFailed     jobRunOutcome = "failed"
	jobRunCanceled   jobRunOutcome = "canceled"
)

func (c *Coordinator) runScheduler(ctx context.Context, lease store.MaintenanceLease) {
	jobs := make([]scheduledJob, len(c.jobs))
	now := time.Now()
	for i, job := range c.jobs {
		jobs[i] = scheduledJob{Job: job, next: now}
	}

	for len(jobs) > 0 {
		next := 0
		for i := 1; i < len(jobs); i++ {
			if jobs[i].next.Before(jobs[next].next) {
				next = i
			}
		}
		if !waitUntil(ctx, jobs[next].next) {
			return
		}
		outcome := c.runJob(ctx, lease, jobs[next].Job)
		if outcome == jobRunCanceled {
			return
		}
		delay := jobs[next].Interval
		if outcome == jobRunIncomplete && c.cfg.ContinuationInterval < delay {
			delay = c.cfg.ContinuationInterval
		}
		jobs[next].next = time.Now().Add(delay)
	}
}

func (c *Coordinator) runJob(ctx context.Context, lease store.MaintenanceLease, job Job) jobRunOutcome {
	started := time.Now()
	cutoff := started.UTC()
	passCtx, cancel := context.WithTimeout(ctx, c.cfg.PassTimeout)
	defer cancel()

	var total int64
	outcome := jobRunCompleted
	for batchNumber := range c.cfg.MaxBatches {
		rows, more, owned, err := c.store.RunMaintenanceBatch(
			passCtx,
			lease,
			func(batchCtx context.Context) (int64, bool, error) {
				return job.RunBatch(batchCtx, cutoff)
			},
		)
		total += rows
		if err != nil {
			switch {
			case ctx.Err() != nil:
				outcome = jobRunCanceled
			case errors.Is(err, context.DeadlineExceeded):
				outcome = jobRunIncomplete
				c.logger.InfoContext(ctx, "cleanup pass reached its time budget", "job", job.Name, "rows", total)
			default:
				outcome = jobRunFailed
				c.logger.ErrorContext(ctx, "cleanup pass failed", "job", job.Name, "rows", total, "error", err)
			}
			break
		}
		if !owned {
			outcome = jobRunCanceled
			c.observeLease("lost")
			c.logger.WarnContext(ctx, "cleanup leadership fencing check failed", "job", job.Name, "generation", lease.Generation)
			break
		}
		if !more {
			break
		}
		if passCtx.Err() != nil {
			outcome = jobRunIncomplete
			c.logger.InfoContext(ctx, "cleanup pass reached its time budget", "job", job.Name, "rows", total)
			break
		}
		if batchNumber == c.cfg.MaxBatches-1 {
			outcome = jobRunIncomplete
			c.logger.InfoContext(ctx, "cleanup pass reached its batch budget", "job", job.Name, "rows", total)
			break
		}
	}

	if c.metrics != nil {
		c.metrics.ObserveMaintenanceRun(job.Name, string(outcome), time.Since(started), total)
	}
	if outcome == jobRunCompleted && total > 0 {
		c.logger.InfoContext(ctx, "cleanup pass completed", "job", job.Name, "rows", total)
	}
	return outcome
}

func (c *Coordinator) observeLease(outcome string) {
	if c.metrics != nil {
		c.metrics.ObserveMaintenanceLease(outcome)
	}
}

func (c *Coordinator) setLeader(leader bool) {
	if c.metrics != nil {
		c.metrics.SetMaintenanceLeader(leader)
	}
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func waitUntil(ctx context.Context, deadline time.Time) bool {
	delay := time.Until(deadline)
	if delay <= 0 {
		return ctx.Err() == nil
	}
	return wait(ctx, delay)
}
