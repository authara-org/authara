package observability

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

	metrics := scrape(t, service)
	assertContains(t, metrics, `go_sql_open_connections{db_name="primary"} 0`)
	assertContains(t, metrics, `authara_background_jobs_total{outcome="retried",worker="email"} 1`)
	assertContains(t, metrics, `authara_background_jobs_total{outcome="succeeded",worker="webhook"} 1`)
	assertContains(t, metrics, `authara_background_job_duration_seconds_count{outcome="retried",worker="email"} 1`)
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
