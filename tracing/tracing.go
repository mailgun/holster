package tracing

import (
	"context"
	"os"
	"runtime/debug"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/sirupsen/logrus"
	"github.com/uptrace/opentelemetry-go-extra/otellogrus"

	"github.com/mailgun/holster/v4/errors"
)

type initState struct {
	opts  []sdktrace.TracerProviderOption
	level Level
}

var (
	logLevels = []logrus.Level{
		logrus.PanicLevel, logrus.FatalLevel, logrus.ErrorLevel,
		logrus.WarnLevel, logrus.InfoLevel, logrus.DebugLevel,
		logrus.TraceLevel,
	}
	log                = logrus.WithField("category", "tracing")
	globalLibraryName  string
	SemconvSchemaURL   = semconv.SchemaURL
	GlobalTestExporter *TestExporter
)

// InitTracing initializes a global OpenTelemetry tracer provider singleton.
// Call to initialize before using functions in this package.
// Instruments logrus to mirror to active trace.  Must use `WithContext()`
// method.
// Call after initializing logrus.
// libraryName is typically the application's module name.  Pass empty string to autodetect module name.
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
		case "test":
			// Used in tests.
			exporter = makeTestExporter()
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

	if libraryName == "" {
		libraryName = getMainModule()
	}
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

func getMainModule() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}

	return info.Main.Path
}

// NewResource creates a resource with sensible defaults.
// Replaces common use case of verbose usage.
func NewResource(serviceName, version string, resources ...*resource.Resource) (*resource.Resource, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			SemconvSchemaURL,
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

	logFields := logrus.Fields{
		"exporter": "otlp",
		"protocol": protocol,
		"endpoint": getenvOrDefault("", "OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"),
	}

	sampler := getenvOrDefault("", "OTEL_TRACES_SAMPLER")
	logFields["sampler"] = sampler
	if strings.HasSuffix(sampler, "traceidratio") {
		logFields["sampler.ratio"], _ = strconv.ParseFloat(getenvOrDefault("", "OTEL_TRACES_SAMPLER_ARG"), 64)
	}

	log.WithFields(logFields).Info("Initializing OpenTelemetry")

	return otlptrace.New(ctx, client)
}

func makeJaegerExporter() (*jaeger.Exporter, error) {
	var endpointOption jaeger.EndpointOption
	protocol := getenvOrDefault("udp/thrift.compact", "OTEL_EXPORTER_JAEGER_PROTOCOL")

	logFields := logrus.Fields{
		"exporter": "jaeger",
		"protocol": protocol,
	}

	sampler := getenvOrDefault("", "OTEL_TRACES_SAMPLER")
	logFields["sampler"] = sampler
	if strings.HasSuffix(sampler, "traceidratio") {
		logFields["sampler.ratio"], _ = strconv.ParseFloat(getenvOrDefault("", "OTEL_TRACES_SAMPLER_ARG"), 64)
	}

	// OTel Jaeger client doesn't seem to implement the spec for
	// OTEL_EXPORTER_JAEGER_PROTOCOL selection.  So we must.
	// Jaeger endpoint option will parse supported env var configuration.
	switch protocol {
	// TODO: Support for "grpc" protocol. https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/sdk-environment-variables.md#jaeger-exporter
	case "http/thrift.binary":
		logFields["endpoint"] = os.Getenv("OTEL_EXPORTER_JAEGER_ENDPOINT")
		log.WithFields(logFields).Info("Initializing OpenTelemetry")
		endpointOption = jaeger.WithCollectorEndpoint()

	case "udp/thrift.binary":
		logFields["agentHost"] = os.Getenv("OTEL_EXPORTER_JAEGER_AGENT_HOST")
		logFields["agentPort"] = os.Getenv("OTEL_EXPORTER_JAEGER_AGENT_PORT")
		log.WithFields(logFields).Info("Initializing OpenTelemetry")
		endpointOption = jaeger.WithAgentEndpoint()

	case "udp/thrift.compact":
		logFields["agentHost"] = os.Getenv("OTEL_EXPORTER_JAEGER_AGENT_HOST")
		logFields["agentPort"] = os.Getenv("OTEL_EXPORTER_JAEGER_AGENT_PORT")
		log.WithFields(logFields).Info("Initializing OpenTelemetry")
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

func makeTestExporter() sdktrace.SpanExporter {
	GlobalTestExporter = new(TestExporter)
	return GlobalTestExporter
}
