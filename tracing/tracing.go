package tracing

import (
	"context"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/mailgun/holster/v4/errors"
	"github.com/sirupsen/logrus"
	"github.com/uptrace/opentelemetry-go-extra/otellogrus"
)

type initState struct {
	opts  []sdktrace.TracerProviderOption
	level Level
}

var logLevels = []logrus.Level{
	logrus.PanicLevel, logrus.FatalLevel, logrus.ErrorLevel,
	logrus.WarnLevel, logrus.InfoLevel, logrus.DebugLevel,
	logrus.TraceLevel,
}

var log = logrus.WithField("category", "tracing")
var globalLibraryName string

// InitTracing initializes a global OpenTelemetry tracer provider singleton.
// Call to initialize before using functions in this package.
// Instruments logrus to mirror to active trace.  Must use `WithContext()`
// method.
// Call after initializing logrus.
// libraryName is typically the application's module name.
// Prometheus metrics are accessible by registering the metrics at
// `tracing.Metrics`.
func InitTracing(ctx context.Context, libraryName string, opts ...TracingOption) error {
	// Setup exporter.
	var err error
	state := &initState{
		level: Level(logrus.GetLevel()),
	}
	exportersEnv := os.Getenv("OTEL_TRACES_EXPORTER")

	for _, e := range strings.Split(exportersEnv, ",") {
		var exporter sdktrace.SpanExporter

		switch e {
		case "none":
			// No exporter.  Used with unit tests.
			continue
		case "jaeger":
			exporter, err = makeJaegerExporter()
			if err != nil {
				return errors.Wrap(err, "error in makeJaegerExporter")
			}
		default:
			// default assuming "otlp".
			exporter, err = makeOtlpExporter(ctx)
			if err != nil {
				return errors.Wrap(err, "error in makeOtlpExporter")
			}
		}

		exportProcessor := sdktrace.NewBatchSpanProcessor(exporter)

		// Capture Prometheus metrics.
		metricProcessor := NewMetricSpanProcessor(exportProcessor)
		state.opts = append(state.opts, sdktrace.WithSpanProcessor(metricProcessor))
	}

	// Apply options.
	for _, opt := range opts {
		opt.apply(state)
	}

	tp := NewLevelTracerProvider(state.level, state.opts...)
	otel.SetTracerProvider(tp)
	globalLibraryName = libraryName

	// Setup logrus instrumentation.
	// Using logrus.WithContext() will mirror log to embedded span.
	// Using WithFields() also converts to log attributes.
	useLevels := []logrus.Level{}
	for _, l := range logLevels {
		if l <= logrus.Level(state.level) {
			useLevels = append(useLevels, l)
		}
	}

	logrus.AddHook(otellogrus.NewHook(
		otellogrus.WithLevels(useLevels...),
	))

	// Required for trace propagation between services.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return err
}

// NewResource creates a resource with sensible defaults.
// Replaces common use case of verbose usage.
func NewResource(serviceName, version string, resources ...*resource.Resource) (*resource.Resource, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(version),
		),
	)
	if err != nil {
		return nil, errors.Wrap(err, "error in resource.Merge")
	}

	for i, res2 := range resources {
		res, err = resource.Merge(res, res2)
		if err != nil {
			return nil, errors.Wrapf(err, "error in resource.Merge on resources index %d", i)
		}
	}

	return res, nil
}

// CloseTracing closes the global OpenTelemetry tracer provider.
// This allows queued up traces to be flushed.
func CloseTracing(ctx context.Context) error {
	tp, ok := otel.GetTracerProvider().(*LevelTracerProvider)
	if !ok {
		return errors.New("OpenTelemetry global tracer provider has not been initialized")
	}

	err := tp.Shutdown(ctx)
	if err != nil {
		return errors.Wrap(err, "error in tp.Shutdown")
	}

	return nil
}

// Tracer returns a tracer object.
func Tracer(opts ...trace.TracerOption) trace.Tracer {
	return otel.Tracer(globalLibraryName, opts...)
}

func getenvOrDefault(def string, names ...string) string {
	for _, name := range names {
		value := os.Getenv(name)
		if value != "" {
			return value
		}
	}

	return def
}

func makeOtlpExporter(ctx context.Context) (*otlptrace.Exporter, error) {
	protocol := getenvOrDefault("grpc", "OTEL_EXPORTER_OTLP_PROTOCOL", "OTEL_EXPORTER_OTLP_TRACES_PROTOCOL")
	var client otlptrace.Client

	// OTel Jaeger client doesn't seem to implement the spec for
	// OTEL_EXPORTER_OTLP_PROTOCOL selection.  So we must.
	// NewClient will parse supported env var configuration.
	switch protocol {
	case "grpc":
		client = otlptracegrpc.NewClient()
	case "http/protobuf":
		client = otlptracehttp.NewClient()
	default:
		log.WithField("OTEL_EXPORTER_OTLP_PROTOCOL", protocol).
			Error("Unknown OTLP protocol configured")
		protocol = "grpc"
		client = otlptracegrpc.NewClient()
	}

	log.WithFields(logrus.Fields{
		"protocol": protocol,
		"endpoint": getenvOrDefault("", "OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"),
	}).Info("Initializing OTLP exporter")

	return otlptrace.New(ctx, client)
}

func makeJaegerExporter() (*jaeger.Exporter, error) {
	var endpointOption jaeger.EndpointOption
	protocol := getenvOrDefault("udp/thrift.compact", "OTEL_EXPORTER_JAEGER_PROTOCOL")

	// OTel Jaeger client doesn't seem to implement the spec for
	// OTEL_EXPORTER_JAEGER_PROTOCOL selection.  So we must.
	// Jaeger endpoint option will parse supported env var configuration.
	switch protocol {
	// TODO: Support for "grpc" protocol. https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/sdk-environment-variables.md#jaeger-exporter
	case "http/thrift.binary":
		log.WithFields(logrus.Fields{
			"endpoint": os.Getenv("OTEL_EXPORTER_JAEGER_ENDPOINT"),
			"protocol": protocol,
		}).Info("Initializing Jaeger exporter")
		endpointOption = jaeger.WithCollectorEndpoint()

	case "udp/thrift.binary":
		log.WithFields(logrus.Fields{
			"agentHost": os.Getenv("OTEL_EXPORTER_JAEGER_AGENT_HOST"),
			"agentPort": os.Getenv("OTEL_EXPORTER_JAEGER_AGENT_PORT"),
			"protocol":  protocol,
		}).Info("Initializing Jaeger exporter")
		endpointOption = jaeger.WithAgentEndpoint()

	case "udp/thrift.compact":
		log.WithFields(logrus.Fields{
			"agentHost": os.Getenv("OTEL_EXPORTER_JAEGER_AGENT_HOST"),
			"agentPort": os.Getenv("OTEL_EXPORTER_JAEGER_AGENT_PORT"),
			"protocol":  protocol,
		}).Info("Initializing Jaeger exporter")
		endpointOption = jaeger.WithAgentEndpoint()

	default:
		log.WithField("OTEL_EXPORTER_JAEGER_PROTOCOL", protocol).
			Error("Unknown Jaeger protocol configured")
		endpointOption = jaeger.WithAgentEndpoint()
	}

	exp, err := jaeger.New(endpointOption)
	if err != nil {
		return nil, errors.Wrap(err, "error in jaeger.New")
	}

	return exp, nil
}
