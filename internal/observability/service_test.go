package observability

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

func TestMiddlewareRecordsBoundedHTTPMetrics(t *testing.T) {
	service := New("test-version")
	router := chi.NewRouter()
	router.Use(service.Middleware)
	router.Get("/users/{userID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("hello"))
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/users/user-123", nil))
	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, response.Code)
	}

	metrics := scrape(t, service)
	assertContains(t, metrics, `authara_http_requests_total{method="GET",route="/users/{userID}",status="201"} 1`)
	assertContains(t, metrics, `authara_http_request_duration_seconds_count{method="GET",route="/users/{userID}",status="201"} 1`)
	assertContains(t, metrics, `authara_http_response_size_bytes_sum{method="GET",route="/users/{userID}",status="201"} 5`)
	if strings.Contains(metrics, "user-123") {
		t.Fatal("metrics must use route patterns instead of raw paths")
	}
}

func TestMiddlewareBoundsUnknownMethodsAndRoutes(t *testing.T) {
	service := New("test-version")
	router := chi.NewRouter()
	router.Use(service.Middleware)
	router.Get("/known", func(http.ResponseWriter, *http.Request) {})

	methodResponse := httptest.NewRecorder()
	router.ServeHTTP(methodResponse, httptest.NewRequest("CUSTOM", "/known", nil))
	if methodResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, methodResponse.Code)
	}

	notFoundResponse := httptest.NewRecorder()
	router.ServeHTTP(notFoundResponse, httptest.NewRequest(http.MethodGet, "/not-found/secret-value", nil))
	if notFoundResponse.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, notFoundResponse.Code)
	}

	metrics := scrape(t, service)
	assertContains(t, metrics, `authara_http_requests_total{method="OTHER",route="unmatched",status="405"} 1`)
	assertContains(t, metrics, `authara_http_requests_total{method="GET",route="unmatched",status="404"} 1`)
	if strings.Contains(metrics, "secret-value") {
		t.Fatal("unmatched raw paths must not be exposed as metric labels")
	}
}

func TestRegistererAddsApplicationCollectors(t *testing.T) {
	service := New("test-version")
	events := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "authara",
		Subsystem: "test",
		Name:      "events_total",
		Help:      "Test application events.",
	})
	if err := service.Registerer().Register(events); err != nil {
		t.Fatalf("register collector: %v", err)
	}
	events.Inc()

	metrics := scrape(t, service)
	assertContains(t, metrics, "authara_test_events_total 1")
}

func TestDatabaseAndBackgroundMetrics(t *testing.T) {
	service := New("test-version")
	if err := service.RegisterDatabase(new(sql.DB), "primary"); err != nil {
		t.Fatalf("register database collector: %v", err)
	}
	service.ObserveBackgroundJob("email", "retried", 250*time.Millisecond)
	service.ObserveBackgroundJob("webhook", "succeeded", 100*time.Millisecond)
	service.ObserveEmailQueueAge("retried", 25*time.Minute)
	service.ObserveMaintenanceLease("skipped")
	service.SetMaintenanceLeader(true)
	service.ObserveMaintenanceRun("sessions_expired", "incomplete", 50*time.Millisecond, 17)
	service.ObserveMaintenanceRun("operator_audit", "completed", 25*time.Millisecond, 3)
	service.ObserveReadinessCheckDuration("postgres", "succeeded", 25*time.Millisecond)
	service.ObserveReadinessCheckDuration("schema", "failed", 50*time.Millisecond)
	service.SetReadiness(true)
	service.ObserveBackgroundPoll("email", "claimed", 10*time.Millisecond)
	service.ObserveBackgroundPoll("webhook", "failed", 20*time.Millisecond)
	service.ObserveQueueReaper("email", "succeeded", 30*time.Millisecond, 2, 1)
	now := time.Unix(1_700_000_000, 0)
	oldestPending := now.Add(-5 * time.Minute)
	oldestReady := now.Add(-2 * time.Minute)
	service.ObserveQueueSnapshot("email", "succeeded", store.QueueStats{
		States: []store.QueueStateStats{
			{State: store.QueueStatePending, Count: 3, OldestCreatedAt: &oldestPending},
			{State: store.QueueStateRetry, Count: 2},
			{State: store.QueueStateProcessing, Count: 1},
			{State: store.QueueStateFailed, Count: 4},
		},
		OldestReadyAt: &oldestReady,
		Stuck:         1,
	}, now)
	service.ObserveQueueSnapshot("webhook", "failed", store.QueueStats{}, now)
	service.SetRuntimeSettingsRevision(6)
	service.ObserveRuntimeSettingsReconciliation("applied", 40*time.Millisecond, 7)

	metrics := scrape(t, service)
	assertContains(t, metrics, `go_sql_open_connections{db_name="primary"} 0`)
	assertContains(t, metrics, `authara_background_jobs_total{outcome="retried",worker="email"} 1`)
	assertContains(t, metrics, `authara_background_jobs_total{outcome="succeeded",worker="webhook"} 1`)
	assertContains(t, metrics, `authara_background_job_duration_seconds_count{outcome="retried",worker="email"} 1`)
	assertContains(t, metrics, `authara_email_queue_age_seconds_count{outcome="retried"} 1`)
	assertContains(t, metrics, `authara_maintenance_leader 1`)
	assertContains(t, metrics, `authara_maintenance_lease_attempts_total{outcome="skipped"} 1`)
	assertContains(t, metrics, `authara_maintenance_runs_total{job="sessions_expired",outcome="incomplete"} 1`)
	assertContains(t, metrics, `authara_maintenance_run_duration_seconds_count{job="sessions_expired",outcome="incomplete"} 1`)
	assertContains(t, metrics, `authara_maintenance_rows_processed_total{job="sessions_expired"} 17`)
	assertContains(t, metrics, `authara_maintenance_runs_total{job="operator_audit",outcome="completed"} 1`)
	assertContains(t, metrics, `authara_maintenance_rows_processed_total{job="operator_audit"} 3`)
	assertContains(t, metrics, `authara_readiness_checks_total{dependency="postgres",result="succeeded"} 1`)
	assertContains(t, metrics, `authara_readiness_checks_total{dependency="schema",result="failed"} 1`)
	assertContains(t, metrics, `authara_readiness_check_duration_seconds_count{dependency="postgres",result="succeeded"} 1`)
	assertContains(t, metrics, `authara_readiness_dependency_status{dependency="postgres"} 1`)
	assertContains(t, metrics, `authara_readiness_dependency_status{dependency="schema"} 0`)
	assertContains(t, metrics, `authara_readiness_status 1`)
	assertContains(t, metrics, `authara_background_polls_total{result="claimed",worker="email"} 1`)
	assertContains(t, metrics, `authara_background_polls_total{result="failed",worker="webhook"} 1`)
	assertContains(t, metrics, `authara_background_poll_duration_seconds_count{result="claimed",worker="email"} 1`)
	assertContains(t, metrics, `authara_queue_reaper_runs_total{queue="email",result="succeeded"} 1`)
	assertContains(t, metrics, `authara_queue_reaper_jobs_total{outcome="retried",queue="email"} 2`)
	assertContains(t, metrics, `authara_queue_reaper_jobs_total{outcome="failed",queue="email"} 1`)
	assertContains(t, metrics, `authara_queue_jobs{queue="email",state="pending"} 3`)
	assertContains(t, metrics, `authara_queue_jobs{queue="email",state="retry"} 2`)
	assertContains(t, metrics, `authara_queue_jobs{queue="email",state="processing"} 1`)
	assertContains(t, metrics, `authara_queue_jobs{queue="email",state="failed"} 4`)
	assertContains(t, metrics, `authara_queue_oldest_job_age_seconds{queue="email",state="pending"} 300`)
	assertContains(t, metrics, `authara_queue_oldest_ready_age_seconds{queue="email"} 120`)
	assertContains(t, metrics, `authara_queue_stuck_jobs{queue="email"} 1`)
	assertContains(t, metrics, `authara_queue_snapshot_refreshes_total{queue="email",result="succeeded"} 1`)
	assertContains(t, metrics, `authara_queue_snapshot_refreshes_total{queue="webhook",result="failed"} 1`)
	assertContains(t, metrics, `authara_queue_snapshot_last_success_timestamp_seconds{queue="email"} 1.7e+09`)
	assertContains(t, metrics, `authara_runtime_settings_reconciliations_total{result="applied"} 1`)
	assertContains(t, metrics, `authara_runtime_settings_reconciliation_duration_seconds_count{result="applied"} 1`)
	assertContains(t, metrics, `authara_runtime_settings_revision 7`)
}

func TestBackgroundMetricLabelsAreBounded(t *testing.T) {
	service := New("test-version")
	service.ObserveBackgroundJob("user-controlled-worker", "user-controlled-outcome", time.Second)

	metrics := scrape(t, service)
	assertContains(t, metrics, `authara_background_jobs_total{outcome="other",worker="other"} 1`)
	if strings.Contains(metrics, "user-controlled") {
		t.Fatal("unknown background metric labels must be normalized")
	}
}

func TestObserveBackgroundJobIsSafeWhenObservabilityIsDisabled(t *testing.T) {
	var service *Service
	service.ObserveBackgroundJob("webhook", "succeeded", time.Second)
	service.ObserveReadinessCheck("postgres", "succeeded")
	service.SetReadiness(true)
	service.ObserveBackgroundPoll("email", "claimed", time.Second)
	service.ObserveQueueReaper("email", "succeeded", time.Second, 1, 0)
	service.ObserveQueueSnapshot("email", "succeeded", store.QueueStats{}, time.Now())
	service.SetRuntimeSettingsRevision(1)
	service.ObserveRuntimeSettingsReconciliation("unchanged", time.Second, 1)
}

func TestReadinessMetricLabelsAreBounded(t *testing.T) {
	service := New("test-version")
	service.ObserveReadinessCheck("user-controlled-dependency", "user-controlled-result")

	metrics := scrape(t, service)
	assertContains(t, metrics, `authara_readiness_checks_total{dependency="other",result="other"} 1`)
	if strings.Contains(metrics, "user-controlled") {
		t.Fatal("unknown readiness metric labels must be normalized")
	}
}

func TestProductionMetricLabelsAreBounded(t *testing.T) {
	service := New("test-version")
	service.ObserveBackgroundPoll("user-controlled-worker", "user-controlled-result", time.Second)
	service.ObserveQueueReaper("user-controlled-queue", "user-controlled-result", time.Second, 0, 0)
	service.ObserveQueueSnapshot("user-controlled-queue", "succeeded", store.QueueStats{
		States: []store.QueueStateStats{{State: "user-controlled-state", Count: 1}},
	}, time.Now())
	service.ObserveRuntimeSettingsReconciliation("user-controlled-result", time.Second, 1)

	metrics := scrape(t, service)
	assertContains(t, metrics, `authara_background_polls_total{result="other",worker="other"} 1`)
	assertContains(t, metrics, `authara_queue_reaper_runs_total{queue="other",result="other"} 1`)
	assertContains(t, metrics, `authara_queue_jobs{queue="other",state="other"} 1`)
	assertContains(t, metrics, `authara_runtime_settings_reconciliations_total{result="other"} 1`)
	if strings.Contains(metrics, "user-controlled") {
		t.Fatal("unknown production metric labels must be normalized")
	}
}

func TestHandlerExposesRuntimeAndBuildMetrics(t *testing.T) {
	metrics := scrape(t, New("v1.2.3"))

	assertContains(t, metrics, `authara_build_info{version="v1.2.3"} 1`)
	assertContains(t, metrics, "go_goroutines ")
	assertContains(t, metrics, "promhttp_metric_handler_requests_in_flight ")
}

func scrape(t *testing.T, service *Service) string {
	t.Helper()

	response := httptest.NewRecorder()
	service.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("expected metrics status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Fatalf("expected Prometheus text content type, got %q", contentType)
	}
	return response.Body.String()
}

func assertContains(t *testing.T, value, expected string) {
	t.Helper()
	if !strings.Contains(value, expected) {
		t.Fatalf("expected output to contain %q\n\n%s", expected, value)
	}
}
