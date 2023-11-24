package ztelemetry

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/torlangballe/zutil/zlog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"

	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

type OpenTelemetry struct {
	shutdownFuncs []func(context.Context) error
}

// Shutdown calls cleanup functions registered via shutdownFuncs.
// The errors from the calls are joined.
// Each registered cleanup will be invoked once.
func (o *OpenTelemetry) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	var err error
	for _, fn := range o.shutdownFuncs {
		err = errors.Join(err, fn(ctx))
	}
	o.shutdownFuncs = nil
	return err
}

func New(ctx context.Context, serviceName, serviceVersion string) *OpenTelemetry {
	o := &OpenTelemetry{}
	// handleErr calls shutdown for cleanup and makes sure that all errors are returned.
	handleErr := func(inErr error) {
		errors.Join(inErr, o.Shutdown(ctx))
	}

	// Set up resource.
	res, err := newResource(serviceName, serviceVersion)
	if err != nil {
		handleErr(err)
		return nil
	}

	// Set up propagator.
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	// Set up trace provider.
	tracerProvider, err := newTraceProvider(res)
	if err != nil {
		handleErr(err)
		return nil
	}
	o.shutdownFuncs = append(o.shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	// Set up meter provider.
	meterProvider, err := newMeterProvider(res)
	if err != nil {
		handleErr(err)
		return nil
	}
	o.shutdownFuncs = append(o.shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

	return o
}

func newResource(serviceName, serviceVersion string) (*resource.Resource, error) {
	return resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		))
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func newTraceProvider(res *resource.Resource) (*trace.TracerProvider, error) {
	traceExporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, err
	}

	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter,
			// Default is 5s. Set to 1s for demonstrative purposes.
			trace.WithBatchTimeout(time.Second)),
		trace.WithResource(res),
	)
	return traceProvider, nil
}

func newMeterProvider(res *resource.Resource) (*metric.MeterProvider, error) {
	metricExporter, err := stdoutmetric.New()
	if err != nil {
		return nil, err
	}
	prometheusExporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter, metric.WithInterval(3*time.Second))), // Default is 1m. Set to 3s for demonstrative purposes.
		metric.WithReader(prometheusExporter),
	)
	return meterProvider, nil
}

func StartHandlerAndServe(port int) {
	if port == 0 {
		port = 2223
	}
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(":2223", nil) //nolint:gosec // Ignoring G114: Use of net/http serve function that has no support for setting timeouts.
	zlog.AssertNotError(err)
}

/*
// getTLS returns a configuration that enables the use of mutual TLS.
func getTLS() (*tls.Config, error) {
	clientAuth, err := tls.LoadX509KeyPair("./confs/client.crt", "./confs/client.key")
	if err != nil {
		return nil, err
	}

	caCert, err := os.ReadFile("./confs/rootCA.crt")
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	c := &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{clientAuth},
	}

	return c, nil
}
*/
