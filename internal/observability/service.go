package observability

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/authara-org/authara/internal/store"
	"github.com/felixge/httpsnoop"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const unmatchedRoute = "unmatched"

type Service struct {
	registry                 *prometheus.Registry
	handler                  http.Handler
	httpRequests             *prometheus.CounterVec
	httpRequestDuration      *prometheus.HistogramVec
	httpResponseSize         *prometheus.HistogramVec
	httpRequestsInFlight     prometheus.Gauge
	backgroundJobs           *prometheus.CounterVec
	backgroundJobDuration    *prometheus.HistogramVec
	emailQueueAge            *prometheus.HistogramVec
	maintenanceLeader        prometheus.Gauge
	maintenanceLeases        *prometheus.CounterVec
	maintenanceRuns          *prometheus.CounterVec
	maintenanceDuration      *prometheus.HistogramVec
	maintenanceRows          *prometheus.CounterVec
	readinessChecks          *prometheus.CounterVec
	readinessDuration        *prometheus.HistogramVec
	readinessDependency      *prometheus.GaugeVec
	readinessStatus          prometheus.Gauge
	queueJobs                *prometheus.GaugeVec
	queueOldestJobAge        *prometheus.GaugeVec
	queueOldestReadyAge      *prometheus.GaugeVec
	queueStuckJobs           *prometheus.GaugeVec
	queueSnapshotRefresh     *prometheus.CounterVec
	queueSnapshotSuccess     *prometheus.GaugeVec
	backgroundPolls          *prometheus.CounterVec
	backgroundPollDuration   *prometheus.HistogramVec
	backgroundPollSuccess    *prometheus.GaugeVec
	queueReaperRuns          *prometheus.CounterVec
	queueReaperDuration      *prometheus.HistogramVec
	queueReaperJobs          *prometheus.CounterVec
	runtimeReconciliations   *prometheus.CounterVec
	runtimeReconcileDuration *prometheus.HistogramVec
	runtimeRevision          prometheus.Gauge
	runtimeLastSuccess       prometheus.Gauge
}

func New(version string) *Service {
	if version == "" {
		version = "unknown"
	}

	registry := prometheus.NewRegistry()
	httpRequests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "authara",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total number of HTTP requests handled by Authara.",
	}, []string{"method", "route", "status"})
	httpRequestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "authara",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "Duration of HTTP requests handled by Authara.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "route", "status"})
	httpResponseSize := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "authara",
		Subsystem: "http",
		Name:      "response_size_bytes",
		Help:      "Size of HTTP responses returned by Authara.",
		Buckets:   prometheus.ExponentialBuckets(128, 2, 16),
	}, []string{"method", "route", "status"})
	httpRequestsInFlight := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "authara",
		Subsystem: "http",
		Name:      "requests_in_flight",
		Help:      "Number of HTTP requests currently being handled by Authara.",
	})
	backgroundJobs := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "authara",
		Subsystem: "background",
		Name:      "jobs_total",
		Help:      "Total number of background jobs processed by Authara.",
	}, []string{"worker", "outcome"})
	backgroundJobDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "authara",
		Subsystem: "background",
		Name:      "job_duration_seconds",
		Help:      "Time spent processing background jobs in Authara.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"worker", "outcome"})
	emailQueueAge := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "authara",
		Subsystem: "email",
		Name:      "queue_age_seconds",
		Help:      "Age of email jobs when a delivery outcome is recorded.",
		Buckets:   prometheus.ExponentialBuckets(1, 4, 10),
	}, []string{"outcome"})
	maintenanceLeader := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "authara",
		Subsystem: "maintenance",
		Name:      "leader",
		Help:      "Whether this Authara replica currently holds cleanup leadership.",
	})
	maintenanceLeases := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "authara",
		Subsystem: "maintenance",
		Name:      "lease_attempts_total",
		Help:      "Cleanup lease lifecycle events observed by this Authara replica.",
	}, []string{"outcome"})
	maintenanceRuns := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "authara",
		Subsystem: "maintenance",
		Name:      "runs_total",
		Help:      "Cleanup passes performed by Authara.",
	}, []string{"job", "outcome"})
	maintenanceDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "authara",
		Subsystem: "maintenance",
		Name:      "run_duration_seconds",
		Help:      "Duration of cleanup passes performed by Authara.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"job", "outcome"})
	maintenanceRows := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "authara",
		Subsystem: "maintenance",
		Name:      "rows_processed_total",
		Help:      "Root rows committed by Authara cleanup passes; foreign-key cascades are not included.",
	}, []string{"job"})
	readinessChecks := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "authara",
		Subsystem: "readiness",
		Name:      "checks_total",
		Help:      "Dependency readiness checks performed by Authara.",
	}, []string{"dependency", "result"})
	readinessDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "authara", Subsystem: "readiness", Name: "check_duration_seconds",
		Help: "Duration of dependency readiness checks performed by Authara.", Buckets: prometheus.DefBuckets,
	}, []string{"dependency", "result"})
	readinessDependency := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "authara", Subsystem: "readiness", Name: "dependency_status",
		Help: "Last observed readiness state of a configured dependency (1 ready, 0 unavailable).",
	}, []string{"dependency"})
	readinessStatus := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "authara", Subsystem: "readiness", Name: "status",
		Help: "Whether this Authara replica is ready to receive traffic.",
	})
	queueJobs := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "authara", Subsystem: "queue", Name: "jobs",
		Help: "Current number of durable asynchronous jobs by queue and state.",
	}, []string{"queue", "state"})
	queueOldestJobAge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "authara", Subsystem: "queue", Name: "oldest_job_age_seconds",
		Help: "Age of the oldest durable asynchronous job by queue and state.",
	}, []string{"queue", "state"})
	queueOldestReadyAge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "authara", Subsystem: "queue", Name: "oldest_ready_age_seconds",
		Help: "Time since the oldest immediately claimable job became ready.",
	}, []string{"queue"})
	queueStuckJobs := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "authara", Subsystem: "queue", Name: "stuck_jobs",
		Help: "Current number of processing jobs whose lease is stale.",
	}, []string{"queue"})
	queueSnapshotRefresh := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "authara", Subsystem: "queue", Name: "snapshot_refreshes_total",
		Help: "Queue snapshot refresh attempts.",
	}, []string{"queue", "result"})
	queueSnapshotSuccess := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "authara", Subsystem: "queue", Name: "snapshot_last_success_timestamp_seconds",
		Help: "Unix timestamp of the last successful queue snapshot refresh.",
	}, []string{"queue"})
	backgroundPolls := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "authara", Subsystem: "background", Name: "polls_total",
		Help: "Background worker queue polls by result.",
	}, []string{"worker", "result"})
	backgroundPollDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "authara", Subsystem: "background", Name: "poll_duration_seconds",
		Help: "Duration of background worker queue polls.", Buckets: prometheus.DefBuckets,
	}, []string{"worker", "result"})
	backgroundPollSuccess := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "authara", Subsystem: "background", Name: "poll_last_success_timestamp_seconds",
		Help: "Unix timestamp of the last successful background queue poll.",
	}, []string{"worker"})
	queueReaperRuns := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "authara", Subsystem: "queue", Name: "reaper_runs_total",
		Help: "Stale-job reaper runs by queue and result.",
	}, []string{"queue", "result"})
	queueReaperDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "authara", Subsystem: "queue", Name: "reaper_duration_seconds",
		Help: "Duration of stale-job reaper runs.", Buckets: prometheus.DefBuckets,
	}, []string{"queue", "result"})
	queueReaperJobs := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "authara", Subsystem: "queue", Name: "reaper_jobs_total",
		Help: "Stale jobs transitioned by the queue reaper.",
	}, []string{"queue", "outcome"})
	runtimeReconciliations := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "authara", Subsystem: "runtime_settings", Name: "reconciliations_total",
		Help: "Runtime-setting reconciliations by result.",
	}, []string{"result"})
	runtimeReconcileDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "authara", Subsystem: "runtime_settings", Name: "reconciliation_duration_seconds",
		Help: "Duration of runtime-setting reconciliations.", Buckets: prometheus.DefBuckets,
	}, []string{"result"})
	runtimeRevision := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "authara", Subsystem: "runtime_settings", Name: "revision",
		Help: "Runtime-settings revision currently applied by this replica.",
	})
	runtimeLastSuccess := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "authara", Subsystem: "runtime_settings", Name: "last_success_timestamp_seconds",
		Help: "Unix timestamp of the last successful runtime-settings reconciliation.",
	})
	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "authara",
		Name:      "build_info",
		Help:      "Build information for the running Authara instance.",
	}, []string{"version"})
	buildInfo.WithLabelValues(version).Set(1)

	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		buildInfo,
		httpRequests,
		httpRequestDuration,
		httpResponseSize,
		httpRequestsInFlight,
		backgroundJobs,
		backgroundJobDuration,
		emailQueueAge,
		maintenanceLeader,
		maintenanceLeases,
		maintenanceRuns,
		maintenanceDuration,
		maintenanceRows,
		readinessChecks,
		readinessDuration,
		readinessDependency,
		readinessStatus,
		queueJobs,
		queueOldestJobAge,
		queueOldestReadyAge,
		queueStuckJobs,
		queueSnapshotRefresh,
		queueSnapshotSuccess,
		backgroundPolls,
		backgroundPollDuration,
		backgroundPollSuccess,
		queueReaperRuns,
		queueReaperDuration,
		queueReaperJobs,
		runtimeReconciliations,
		runtimeReconcileDuration,
		runtimeRevision,
		runtimeLastSuccess,
	)
	for _, queue := range []string{"email", "webhook"} {
		queueOldestReadyAge.WithLabelValues(queue).Set(0)
		queueStuckJobs.WithLabelValues(queue).Set(0)
		queueSnapshotSuccess.WithLabelValues(queue).Set(0)
		for _, state := range []string{store.QueueStatePending, store.QueueStateRetry, store.QueueStateProcessing, store.QueueStateFailed} {
			queueJobs.WithLabelValues(queue, state).Set(0)
			queueOldestJobAge.WithLabelValues(queue, state).Set(0)
		}
	}

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		Registry:          registry,
		EnableOpenMetrics: true,
	})

	return &Service{
		registry:                 registry,
		handler:                  promhttp.InstrumentMetricHandler(registry, handler),
		httpRequests:             httpRequests,
		httpRequestDuration:      httpRequestDuration,
		httpResponseSize:         httpResponseSize,
		httpRequestsInFlight:     httpRequestsInFlight,
		backgroundJobs:           backgroundJobs,
		backgroundJobDuration:    backgroundJobDuration,
		emailQueueAge:            emailQueueAge,
		maintenanceLeader:        maintenanceLeader,
		maintenanceLeases:        maintenanceLeases,
		maintenanceRuns:          maintenanceRuns,
		maintenanceDuration:      maintenanceDuration,
		maintenanceRows:          maintenanceRows,
		readinessChecks:          readinessChecks,
		readinessDuration:        readinessDuration,
		readinessDependency:      readinessDependency,
		readinessStatus:          readinessStatus,
		queueJobs:                queueJobs,
		queueOldestJobAge:        queueOldestJobAge,
		queueOldestReadyAge:      queueOldestReadyAge,
		queueStuckJobs:           queueStuckJobs,
		queueSnapshotRefresh:     queueSnapshotRefresh,
		queueSnapshotSuccess:     queueSnapshotSuccess,
		backgroundPolls:          backgroundPolls,
		backgroundPollDuration:   backgroundPollDuration,
		backgroundPollSuccess:    backgroundPollSuccess,
		queueReaperRuns:          queueReaperRuns,
		queueReaperDuration:      queueReaperDuration,
		queueReaperJobs:          queueReaperJobs,
		runtimeReconciliations:   runtimeReconciliations,
		runtimeReconcileDuration: runtimeReconcileDuration,
		runtimeRevision:          runtimeRevision,
		runtimeLastSuccess:       runtimeLastSuccess,
	}
}

// Registerer allows application modules to register additional collectors
// without relying on Prometheus's global registry.
func (s *Service) Registerer() prometheus.Registerer {
	return s.registry
}

// RegisterDatabase exposes database/sql pool saturation and connection churn.
func (s *Service) RegisterDatabase(db *sql.DB, name string) error {
	if db == nil {
		return errors.New("database is required")
	}
	if name == "" {
		name = "unknown"
	}
	return s.registry.Register(collectors.NewDBStatsCollector(db, name))
}

// ObserveBackgroundJob records the result and processing time of an asynchronous job.
func (s *Service) ObserveBackgroundJob(worker, outcome string, duration time.Duration) {
	if s == nil {
		return
	}

	worker = normalizeBackgroundWorker(worker)
	outcome = normalizeBackgroundOutcome(outcome)
	s.backgroundJobs.WithLabelValues(worker, outcome).Inc()
	s.backgroundJobDuration.WithLabelValues(worker, outcome).Observe(duration.Seconds())
}

func (s *Service) ObserveEmailQueueAge(outcome string, age time.Duration) {
	if s == nil {
		return
	}
	if age < 0 {
		age = 0
	}
	s.emailQueueAge.WithLabelValues(normalizeBackgroundOutcome(outcome)).Observe(age.Seconds())
}

func (s *Service) ObserveMaintenanceLease(outcome string) {
	if s == nil {
		return
	}
	s.maintenanceLeases.WithLabelValues(normalizeMaintenanceLeaseOutcome(outcome)).Inc()
}

func (s *Service) SetMaintenanceLeader(leader bool) {
	if s == nil {
		return
	}
	if leader {
		s.maintenanceLeader.Set(1)
		return
	}
	s.maintenanceLeader.Set(0)
}

func (s *Service) ObserveMaintenanceRun(job, outcome string, duration time.Duration, rows int64) {
	if s == nil {
		return
	}
	job = normalizeMaintenanceJob(job)
	outcome = normalizeMaintenanceRunOutcome(outcome)
	s.maintenanceRuns.WithLabelValues(job, outcome).Inc()
	s.maintenanceDuration.WithLabelValues(job, outcome).Observe(duration.Seconds())
	if rows > 0 {
		s.maintenanceRows.WithLabelValues(job).Add(float64(rows))
	}
}

func (s *Service) ObserveReadinessCheck(dependency, result string) {
	s.ObserveReadinessCheckDuration(dependency, result, 0)
}

func (s *Service) ObserveReadinessCheckDuration(dependency, result string, duration time.Duration) {
	if s == nil {
		return
	}
	dependency = normalizeReadinessDependency(dependency)
	result = normalizeReadinessResult(result)
	s.readinessChecks.WithLabelValues(dependency, result).Inc()
	s.readinessDuration.WithLabelValues(dependency, result).Observe(max(duration.Seconds(), 0))
	if result == "succeeded" {
		s.readinessDependency.WithLabelValues(dependency).Set(1)
	} else if result == "failed" {
		s.readinessDependency.WithLabelValues(dependency).Set(0)
	}
}

func (s *Service) SetReadiness(ready bool) {
	if s == nil {
		return
	}
	if ready {
		s.readinessStatus.Set(1)
	} else {
		s.readinessStatus.Set(0)
	}
}

func (s *Service) ObserveBackgroundPoll(worker, result string, duration time.Duration) {
	if s == nil {
		return
	}
	worker = normalizeBackgroundWorker(worker)
	result = normalizeBackgroundPollResult(result)
	s.backgroundPolls.WithLabelValues(worker, result).Inc()
	s.backgroundPollDuration.WithLabelValues(worker, result).Observe(max(duration.Seconds(), 0))
	if result == "claimed" || result == "empty" {
		s.backgroundPollSuccess.WithLabelValues(worker).Set(float64(time.Now().Unix()))
	}
}

func (s *Service) ObserveQueueReaper(queue, result string, duration time.Duration, retried, failed int64) {
	if s == nil {
		return
	}
	queue = normalizeQueue(queue)
	result = normalizeOperationResult(result)
	s.queueReaperRuns.WithLabelValues(queue, result).Inc()
	s.queueReaperDuration.WithLabelValues(queue, result).Observe(max(duration.Seconds(), 0))
	if retried > 0 {
		s.queueReaperJobs.WithLabelValues(queue, "retried").Add(float64(retried))
	}
	if failed > 0 {
		s.queueReaperJobs.WithLabelValues(queue, "failed").Add(float64(failed))
	}
}

func (s *Service) ObserveQueueSnapshot(queue, result string, stats store.QueueStats, now time.Time) {
	if s == nil {
		return
	}
	queue = normalizeQueue(queue)
	result = normalizeOperationResult(result)
	s.queueSnapshotRefresh.WithLabelValues(queue, result).Inc()
	if result != "succeeded" {
		return
	}
	for _, state := range []string{store.QueueStatePending, store.QueueStateRetry, store.QueueStateProcessing, store.QueueStateFailed} {
		s.queueJobs.WithLabelValues(queue, state).Set(0)
		s.queueOldestJobAge.WithLabelValues(queue, state).Set(0)
	}
	for _, state := range stats.States {
		stateName := normalizeQueueState(state.State)
		s.queueJobs.WithLabelValues(queue, stateName).Set(float64(max(state.Count, 0)))
		age := 0.0
		if state.OldestCreatedAt != nil {
			age = max(now.Sub(*state.OldestCreatedAt).Seconds(), 0)
		}
		s.queueOldestJobAge.WithLabelValues(queue, stateName).Set(age)
	}
	readyAge := 0.0
	if stats.OldestReadyAt != nil {
		readyAge = max(now.Sub(*stats.OldestReadyAt).Seconds(), 0)
	}
	s.queueOldestReadyAge.WithLabelValues(queue).Set(readyAge)
	s.queueStuckJobs.WithLabelValues(queue).Set(float64(max(stats.Stuck, 0)))
	s.queueSnapshotSuccess.WithLabelValues(queue).Set(float64(now.Unix()))
}

func (s *Service) SetRuntimeSettingsRevision(revision int64) {
	if s != nil {
		s.runtimeRevision.Set(float64(max(revision, 0)))
	}
}

func (s *Service) ObserveRuntimeSettingsReconciliation(result string, duration time.Duration, revision int64) {
	if s == nil {
		return
	}
	result = normalizeRuntimeReconcileResult(result)
	s.runtimeReconciliations.WithLabelValues(result).Inc()
	s.runtimeReconcileDuration.WithLabelValues(result).Observe(max(duration.Seconds(), 0))
	if result == "applied" || result == "unchanged" {
		s.runtimeRevision.Set(float64(max(revision, 0)))
		s.runtimeLastSuccess.Set(float64(time.Now().Unix()))
	}
}

func (s *Service) Handler() http.Handler {
	return s.handler
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.httpRequestsInFlight.Inc()
		defer s.httpRequestsInFlight.Dec()

		metrics := httpsnoop.CaptureMetrics(next, w, r)
		labels := []string{
			normalizeMethod(r.Method),
			routePattern(r),
			strconv.Itoa(metrics.Code),
		}
		s.httpRequests.WithLabelValues(labels...).Inc()
		s.httpRequestDuration.WithLabelValues(labels...).Observe(metrics.Duration.Seconds())
		s.httpResponseSize.WithLabelValues(labels...).Observe(float64(metrics.Written))
	})
}

func routePattern(r *http.Request) string {
	if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
		if pattern := routeContext.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return unmatchedRoute
}

func normalizeMethod(method string) string {
	switch method {
	case http.MethodConnect,
		http.MethodDelete,
		http.MethodGet,
		http.MethodHead,
		http.MethodOptions,
		http.MethodPatch,
		http.MethodPost,
		http.MethodPut,
		http.MethodTrace:
		return method
	default:
		return "OTHER"
	}
}

func normalizeBackgroundWorker(worker string) string {
	switch worker {
	case "email", "webhook":
		return worker
	default:
		return "other"
	}
}

func normalizeBackgroundOutcome(outcome string) string {
	switch outcome {
	case "succeeded", "retried", "failed", "error":
		return outcome
	default:
		return "other"
	}
}

func normalizeBackgroundPollResult(result string) string {
	switch result {
	case "claimed", "empty", "failed":
		return result
	default:
		return "other"
	}
}

func normalizeQueue(queue string) string {
	switch queue {
	case "email", "webhook":
		return queue
	default:
		return "other"
	}
}

func normalizeQueueState(state string) string {
	switch state {
	case store.QueueStatePending, store.QueueStateRetry, store.QueueStateProcessing, store.QueueStateFailed:
		return state
	default:
		return "other"
	}
}

func normalizeOperationResult(result string) string {
	switch result {
	case "succeeded", "failed":
		return result
	default:
		return "other"
	}
}

func normalizeRuntimeReconcileResult(result string) string {
	switch result {
	case "applied", "unchanged", "failed":
		return result
	default:
		return "other"
	}
}

func normalizeMaintenanceJob(job string) string {
	switch job {
	case "sessions_expired", "sessions_revoked", "refresh_tokens", "webauthn_challenges",
		"email_sent", "email_failed", "challenges", "email_verifications",
		"webhook", "admin_audit", "operator_audit", "security_events":
		return job
	default:
		return "other"
	}
}

func normalizeMaintenanceRunOutcome(outcome string) string {
	switch outcome {
	case "completed", "incomplete", "failed", "canceled":
		return outcome
	default:
		return "other"
	}
}

func normalizeMaintenanceLeaseOutcome(outcome string) string {
	switch outcome {
	case "acquired", "skipped", "released", "lost", "failed":
		return outcome
	default:
		return "other"
	}
}

func normalizeReadinessDependency(dependency string) string {
	switch dependency {
	case "postgres", "schema", "redis":
		return dependency
	default:
		return "other"
	}
}

func normalizeReadinessResult(result string) string {
	switch result {
	case "succeeded", "failed":
		return result
	default:
		return "other"
	}
}
