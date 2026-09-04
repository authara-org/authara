package observability

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const unmatchedRoute = "unmatched"

type Service struct {
	registry              *prometheus.Registry
	handler               http.Handler
	httpRequests          *prometheus.CounterVec
	httpRequestDuration   *prometheus.HistogramVec
	httpResponseSize      *prometheus.HistogramVec
	httpRequestsInFlight  prometheus.Gauge
	backgroundJobs        *prometheus.CounterVec
	backgroundJobDuration *prometheus.HistogramVec
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
	)

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		Registry:          registry,
		EnableOpenMetrics: true,
	})

	return &Service{
		registry:              registry,
		handler:               promhttp.InstrumentMetricHandler(registry, handler),
		httpRequests:          httpRequests,
		httpRequestDuration:   httpRequestDuration,
		httpResponseSize:      httpResponseSize,
		httpRequestsInFlight:  httpRequestsInFlight,
		backgroundJobs:        backgroundJobs,
		backgroundJobDuration: backgroundJobDuration,
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
	worker = normalizeBackgroundWorker(worker)
	outcome = normalizeBackgroundOutcome(outcome)
	s.backgroundJobs.WithLabelValues(worker, outcome).Inc()
	s.backgroundJobDuration.WithLabelValues(worker, outcome).Observe(duration.Seconds())
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
