package middleware

import (
    "net/http"
    "strconv"
    "strings"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    requestCount = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "app_request_total",
            Help: "Total requests by entity, operation and status class",
        },
        []string{"entity", "operation", "status_class"},
    )

    // Гистограмма для общего распределения (можно использовать для расчёта квантилей на сервере метрик)
    requestDurationHistogram = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "app_request_duration_seconds",
            Help:    "Request duration in seconds (histogram)",
            Buckets: prometheus.DefBuckets,
        },
        []string{"entity", "operation"},
    )

    // Summary с вычисляемыми процентилями (квантилями)
    requestDurationSummary = promauto.NewSummaryVec(
        prometheus.SummaryOpts{
            Name:       "app_request_duration_summary_seconds",
            Help:       "Request duration in seconds (summary with quantiles)",
            Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
        },
        []string{"entity", "operation"},
    )

    clientErrors = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "app_client_errors_total",
            Help: "Total client errors (4xx) by entity and operation",
        },
        []string{"entity", "operation"},
    )
    serverErrors = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "app_server_errors_total",
            Help: "Total server errors (5xx) by entity and operation",
        },
        []string{"entity", "operation"},
    )
)

func getLabels(r *http.Request) (entity, operation string) {
    path := r.URL.Path
    method := r.Method

    if strings.HasPrefix(path, "/api/users") {
        entity = "user"
    } else if strings.HasPrefix(path, "/api/profile") {
        entity = "profile"
    } else {
        entity = "other"
    }

    switch method {
    case http.MethodGet:
        operation = "get"
    case http.MethodPost:
        operation = "create"
    case http.MethodPut:
        operation = "update"
    case http.MethodDelete:
        operation = "delete"
    default:
        operation = strings.ToLower(method)
    }
    return
}

type responseWriter struct {
    http.ResponseWriter
    status int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.status = code
    rw.ResponseWriter.WriteHeader(code)
}

func MetricsMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if r.URL.Path == "/metrics" {
                next.ServeHTTP(w, r)
                return
            }

            start := time.Now()
            rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

            entity, operation := getLabels(r)

            next.ServeHTTP(rw, r)

            dur := time.Since(start).Seconds()
            statusClass := strconv.Itoa(rw.status/100) + "xx"

            requestCount.WithLabelValues(entity, operation, statusClass).Inc()
            requestDurationHistogram.WithLabelValues(entity, operation).Observe(dur)
            requestDurationSummary.WithLabelValues(entity, operation).Observe(dur)

            if rw.status >= 400 && rw.status < 500 {
                clientErrors.WithLabelValues(entity, operation).Inc()
            } else if rw.status >= 500 {
                serverErrors.WithLabelValues(entity, operation).Inc()
            }
        })
    }
}