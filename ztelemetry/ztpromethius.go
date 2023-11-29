package ztelemetry

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Counter = prometheus.Counter
type Gauge = prometheus.Gauge
type GaugeVec = prometheus.GaugeVec
type Histogram = prometheus.Histogram

var (
	registry    prometheus.Registerer
	httpBuckets []float64 = prometheus.ExponentialBuckets(0.1, 1.5, 5)
)

func IsRunning() bool {
	return registry != nil
}

func StartPromethiusHandling(port int) {
	if port == 0 {
		port = 9090
	}
	router := mux.NewRouter()

	reg := prometheus.NewRegistry()
	registry = reg

	// Add go runtime metrics and process collectors.
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	// Expose /metrics HTTP endpoint using the created custom registry.
	router.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	// registry = prometheus.NewRegistry()

	// // Add go runtime metrics and process collectors.
	// registry.MustRegister(
	// 	collectors.NewGoCollector(),
	// 	collectors.NewProcessCollector(collectors.ProcessCollectorOpts{Registry: registry}),
	// )

	// router.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	http.ListenAndServe(fmt.Sprint(":", port), router)
}

func NewCounter(name, help string) Counter {
	c := promauto.NewCounter(prometheus.CounterOpts{
		Name: name,
		Help: help,
	})
	registry.MustRegister(c)
	return c
}

func NewGauge(name, help string) Gauge {
	g := promauto.NewGauge(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	})
	registry.MustRegister(g)
	return g
}

func NewGaugeVec(name, help string, labelNames ...string) *GaugeVec {
	g := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	}, labelNames)
	registry.MustRegister(g)
	return g
}

func NewHistogram(name, help string, buckets []float64, labelNames ...string) Histogram {
	h := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "zui",
			Name:      name,
			Buckets:   buckets,
			Help:      help,
		}) //.With(labels)
	return h
}

func GaugeVecSetWithLabels(g GaugeVec, val float64, labels map[string]string) {
	g.With(labels).Set(val)
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
