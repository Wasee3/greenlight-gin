package main

import "github.com/prometheus/client_golang/prometheus"

// Define Prometheus Metrics
var (
	HttpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests received per handler",
		},
		[]string{"method", "endpoint", "status"},
	)

	HttpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Request duration per handler",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	HttpRequestSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "Size of incoming requests",
			Buckets: prometheus.ExponentialBuckets(100, 2, 10), // Custom buckets for request sizes
		},
		[]string{"method", "endpoint"},
	)

	HttpResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "Size of outgoing responses",
			Buckets: prometheus.ExponentialBuckets(100, 2, 10),
		},
		[]string{"method", "endpoint"},
	)

	HttpRequestsErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_errors_total",
			Help: "Total count of failed requests (4xx, 5xx)",
		},
		[]string{"method", "endpoint", "status"},
	)

	DbQueryErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_query_errors_total",
			Help: "Number of failed database queries",
		},
		[]string{"query_type"},
	)

	PanicRecoveryTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "panic_recovery_total",
			Help: "Number of panics recovered in the middleware",
		},
		[]string{"handler"},
	)

	DbQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Time taken to execute database queries",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"query_type"},
	)

	DbConnectionsActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_connections_active",
			Help: "Number of active database connections",
		},
		[]string{"db_instance"},
	)

	UserRegistrationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "user_registrations_total",
			Help: "Total number of new user registrations",
		},
		[]string{"status"},
	)

	LoginsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "logins_total",
			Help: "Number of successful user logins",
		},
		[]string{"method"},
	)

	FailedLoginsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "failed_logins_total",
			Help: "Number of failed logins by reason (invalid password, blocked user, etc.)",
		},
		[]string{"reason"},
	)

	GoGoroutines = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "go_goroutines",
			Help: "Number of currently running Goroutines",
		},
	)

	GoMemAllocBytes = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "go_mem_alloc_bytes",
			Help: "Memory allocated in bytes",
		},
	)

	GoMemHeapObjects = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "go_mem_heap_objects",
			Help: "Number of heap objects allocated",
		},
	)
)
