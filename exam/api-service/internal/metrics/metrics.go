package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ReqTotal – общий счётчик HTTP-запросов с метками method, path, status
var ReqTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "api_requests_total",
		Help: "Total API requests",
	},
	[]string{"method", "path", "status"},
)