// Package metrics exposes process and request metrics for scraping.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type Registry struct {
	*prometheus.Registry

	requests *prometheus.CounterVec
	latency  *prometheus.HistogramVec

	cacheEvents *prometheus.CounterVec
}

// New builds a registry with the Go runtime and process collectors.
func New() *Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Requests by route, method and status.",
	}, []string{"route", "method", "status"})

	latency := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Request duration by route and method.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"route", "method"})

	cacheEvents := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_events_total",
		Help: "Cache hits, misses and invalidations by scope kind.",
	}, []string{"kind", "event"})

	reg.MustRegister(requests, latency, cacheEvents)
	return &Registry{Registry: reg, requests: requests, latency: latency, cacheEvents: cacheEvents}
}

// Middleware records the count and duration of requests per matched route.
func (r *Registry) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		wrapped := middleware.NewWrapResponseWriter(w, req.ProtoMajor)
		started := time.Now()

		next.ServeHTTP(wrapped, req)

		route := chiRoutePattern(req)
		r.requests.WithLabelValues(route, req.Method, strconv.Itoa(wrapped.Status())).Inc()
		r.latency.WithLabelValues(route, req.Method).Observe(time.Since(started).Seconds())
	})
}

// RecordCache counts a cache hit, miss or invalidation for a scope kind.
func (r *Registry) RecordCache(kind, event string) {
	r.cacheEvents.WithLabelValues(kind, event).Inc()
}

func chiRoutePattern(req *http.Request) string {
	if ctx := chiContext(req); ctx != "" {
		return ctx
	}
	return "unmatched"
}

func PoolCollector(pool *pgxpool.Pool) prometheus.Collector {
	acquired := prometheus.NewDesc("pgxpool_acquired_conns", "Connections in use.", nil, nil)
	idle := prometheus.NewDesc("pgxpool_idle_conns", "Idle connections.", nil, nil)
	total := prometheus.NewDesc("pgxpool_total_conns", "Open connections.", nil, nil)
	maximum := prometheus.NewDesc("pgxpool_max_conns", "Pool ceiling.", nil, nil)
	waited := prometheus.NewDesc("pgxpool_acquire_wait_seconds_total",
		"Total time spent waiting for a connection.", nil, nil)

	return &poolCollector{
		pool: pool, acquired: acquired, idle: idle,
		total: total, maximum: maximum, waited: waited,
	}
}

type poolCollector struct {
	pool                                   *pgxpool.Pool
	acquired, idle, total, maximum, waited *prometheus.Desc
}

func (c *poolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.acquired
	ch <- c.idle
	ch <- c.total
	ch <- c.maximum
	ch <- c.waited
}

func (c *poolCollector) Collect(ch chan<- prometheus.Metric) {
	stat := c.pool.Stat()
	ch <- prometheus.MustNewConstMetric(c.acquired, prometheus.GaugeValue, float64(stat.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(c.idle, prometheus.GaugeValue, float64(stat.IdleConns()))
	ch <- prometheus.MustNewConstMetric(c.total, prometheus.GaugeValue, float64(stat.TotalConns()))
	ch <- prometheus.MustNewConstMetric(c.maximum, prometheus.GaugeValue, float64(stat.MaxConns()))
	ch <- prometheus.MustNewConstMetric(c.waited, prometheus.CounterValue, stat.AcquireDuration().Seconds())
}
