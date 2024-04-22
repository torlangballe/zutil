//go:build server

package ztelemetry

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/torlangballe/zutil/zlog"
)

type CounterVec struct {
	cv         *prometheus.CounterVec
	registered bool
}

type GaugeVec struct {
	gv         *prometheus.GaugeVec
	registered bool
}

type HistogramVec struct {
	hv         *prometheus.HistogramVec
	registered bool
}

type SummaryVec struct {
	sv         *prometheus.SummaryVec
	registered bool
}

const URLBaseLabel = "url_base"

var (
	registry    *prometheus.Registry
	httpBuckets []float64 = prometheus.ExponentialBuckets(0.1, 1.5, 5)
)

func IsRunning() bool {
	return registry != nil
}

func StartPrometheusHandling() {
	registry = prometheus.NewRegistry()
	port := PrometheusPort.Get()
	if port == 0 {
		zlog.Info("Prometheus: No port")
		return
	}
	router := mux.NewRouter()
	// Add go runtime metrics and process collectors.
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	zlog.Info("Hosting Prometheus Scraping on port", port)
	// Expose /metrics HTTP endpoint using the created custom registry.
	router.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry}))

	// registry = prometheus.NewRegistry()

	// // Add go runtime metrics and process collectors.
	// registry.MustRegister(
	// 	collectors.NewGoCollector(),
	// 	collectors.NewProcessCollector(collectors.ProcessCollectorOpts{Registry: registry}),
	// )

	// router.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	go http.ListenAndServe(fmt.Sprint(":", port), router)
}

func NewCounterVec(name, help string, labelNames ...string) *CounterVec {
	var c CounterVec

	c.cv = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: name,
		Help: help,
	}, labelNames)
	return &c
}

func (c *CounterVec) Inc(labels map[string]string) {
	if !c.registered {
		c.registered = true
		registry.MustRegister(c.cv)
	}
	c.cv.With(labels).Inc()
}

func NewGaugeVec(name, help string, labelNames ...string) *GaugeVec {
	var g GaugeVec
	g.gv = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	}, labelNames)
	return &g
}

func (g *GaugeVec) Set(val float64, labels map[string]string) {
	if !g.registered {
		g.registered = true
		registry.MustRegister(g.gv)
	}
	g.gv.With(labels).Set(val)
}

func NewHistogramVec(name string, buckets []float64, help string, labelNames ...string) *HistogramVec {
	var h HistogramVec
	h.hv = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			// Namespace: "zui",
			Name:    name,
			Buckets: buckets,
			Help:    help,
		}, labelNames)
	return &h
}

func (h *HistogramVec) Observe(val float64, labels map[string]string) {
	if !h.registered {
		h.registered = true
		registry.MustRegister(h.hv)
	}
	h.hv.With(labels).Observe(val)
}

func NewSummaryVec(name string, help string, labelNames ...string) *SummaryVec {
	var s SummaryVec
	s.sv = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			// Namespace: "zui",
			Name: name,
			Help: help,
		}, labelNames)
	return &s
}

func (s *SummaryVec) Observe(val float64, labels map[string]string) {
	if !s.registered {
		s.registered = true
		registry.MustRegister(s.sv)
	}
	s.sv.With(labels).Observe(val)
}

func WrapHandler(handlerName string, handlerFunc http.HandlerFunc) http.HandlerFunc {
	reg := prometheus.WrapRegistererWith(prometheus.Labels{"handler": handlerName}, registry)
	requestsTotal := promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Tracks the number of HTTP requests.",
		}, []string{"method", "code"},
	)
	requestDuration := promauto.With(reg).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Tracks the latencies for HTTP requests.",
			Buckets: httpBuckets,
		},
		[]string{"method", "code"},
	)
	requestSize := promauto.With(reg).NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "http_request_size_bytes",
			Help: "Tracks the size of HTTP requests.",
		},
		[]string{"method", "code"},
	)
	responseSize := promauto.With(reg).NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "http_response_size_bytes",
			Help: "Tracks the size of HTTP responses.",
		},
		[]string{"method", "code"},
	)

	// Wraps the provided http.Handler to observe the request result with the provided metrics.
	base := promhttp.InstrumentHandlerCounter(
		requestsTotal,
		promhttp.InstrumentHandlerDuration(
			requestDuration,
			promhttp.InstrumentHandlerRequestSize(
				requestSize,
				promhttp.InstrumentHandlerResponseSize(
					responseSize,
					http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
						handlerFunc(writer, r)
					}),
				),
			),
		),
	)

	return base.ServeHTTP
}
