package tracing

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/mailgun/holster/v4/errors"
	"github.com/sirupsen/logrus"
	"github.com/uptrace/opentelemetry-go-extra/otellogrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"
)

// Context Values key for embedding `Tracer` object.
type tracerKey struct{}

var logLevels = []logrus.Level{
	logrus.PanicLevel, logrus.FatalLevel, logrus.ErrorLevel,
	logrus.WarnLevel, logrus.InfoLevel, logrus.DebugLevel,
	logrus.TraceLevel,
}

var log = logrus.WithField("category", "tracing")
var globalMutex sync.RWMutex
var defaultTracer trace.Tracer

// InitTracing initializes a global OpenTelemetry tracer provider singleton.
// Call once before using functions in this package.
// Embeds `Tracer` object in returned context.
// Instruments logrus to mirror to active trace.  Must use `WithContext()`
// method.
// Call after initializing logrus.
// libraryName is typically the application's module name.
func InitTracing(ctx context.Context, libraryName string, opts ...sdktrace.TracerProviderOption) (context.Context, trace.Tracer, error) {
	exp, err := makeJaegerExporter()
	if err != nil {
		return ctx, nil, errors.Wrap(err, "error in makeJaegerExporter")
	}

	opts2 := []sdktrace.TracerProviderOption{
		sdktrace.WithBatcher(exp),
	}
	opts2 = append(opts2, opts...)

	tp := sdktrace.NewTracerProvider(opts2...)
	otel.SetTracerProvider(tp)

	// Setup logrus instrumentation.
	// Using logrus.WithContext() will mirror log to embedded span.
	// Using WithFields() also converts to log attributes.
	logLevel := logrus.GetLevel()
	useLevels := []logrus.Level{}
	for _, l := range logLevels {
		if l <= logLevel {
			useLevels = append(useLevels, l)
		}
	}

	logrus.AddHook(otellogrus.NewHook(
		otellogrus.WithLevels(useLevels...),
	))

	tracerCtx, tracer, err := NewTracer(ctx, libraryName)
	if err != nil {
		return ctx, nil, errors.Wrap(err, "error in NewTracer")
	}

	if GetDefaultTracer() == nil {
		SetDefaultTracer(tracer)
	}

	// Required for trace propagation between services.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return tracerCtx, tracer, err
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

// NewTracer instantiates a new `Tracer` object with a custom library name.
// Must call `InitTracing()` first.
// Library name is set in span attribute `otel.library.name`.
// This is typically the relevant package name.
func NewTracer(ctx context.Context, libraryName string) (context.Context, trace.Tracer, error) {
	tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	if !ok {
		return nil, nil, errors.New("OpenTelemetry global tracer provider has not been initialized")
	}

	tracer := tp.Tracer(libraryName)
	ctx = ContextWithTracer(ctx, tracer)
	return ctx, tracer, nil
}

// GetDefaultTracer gets the global tracer used as a default by this package.
func GetDefaultTracer() trace.Tracer {
	globalMutex.RLock()
	defer globalMutex.RUnlock()
	return defaultTracer
}

// SetDefaultTracer sets the global tracer used as a default by this package.
func SetDefaultTracer(tracer trace.Tracer) {
	globalMutex.Lock()
	defer globalMutex.Unlock()
	defaultTracer = tracer
}

// CloseTracing closes the global OpenTelemetry tracer provider.
// This allows queued up traces to be flushed.
func CloseTracing(ctx context.Context) error {
	tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	if !ok {
		return errors.New("OpenTelemetry global tracer provider has not been initialized")
	}

	SetDefaultTracer(nil)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := tp.Shutdown(ctx)
	if err != nil {
		return errors.Wrap(err, "error in tp.Shutdown")
	}

	return nil
}

// ContextWithTracer creates a context with a tracer object embedded.
// This value is used by scope functions or use TracerFromContext() to retrieve
// it.
func ContextWithTracer(ctx context.Context, tracer trace.Tracer) context.Context {
	return context.WithValue(ctx, tracerKey{}, tracer)
}

// TracerFromContext gets embedded `Tracer` from context.
// Returns nil if not found.
func TracerFromContext(ctx context.Context) trace.Tracer {
	tracer, _ := ctx.Value(tracerKey{}).(trace.Tracer)
	return tracer
}

func makeJaegerExporter() (*jaeger.Exporter, error) {
	var endpointOption jaeger.EndpointOption

	protocol := os.Getenv("OTEL_EXPORTER_JAEGER_PROTOCOL")
	if protocol == "" {
		protocol = "udp/thrift.compact"
	}

	// Otel Jaeger client doesn't seem to implement the spec for
	// OTEL_EXPORTER_JAEGER_PROTOCOL selection.  So we must.
	switch protocol {
	case "http/thrift.binary":
		log.WithFields(logrus.Fields{
			"endpoint": os.Getenv("OTEL_EXPORTER_JAEGER_ENDPOINT"),
		}).Infof("Initializing Jaeger exporter via protocol %s", protocol)
		endpointOption = jaeger.WithCollectorEndpoint()

	case "udp/thrift.binary":
		log.WithFields(logrus.Fields{
			"agentHost": os.Getenv("OTEL_EXPORTER_JAEGER_AGENT_HOST"),
			"agentPort": os.Getenv("OTEL_EXPORTER_JAEGER_AGENT_PORT"),
		}).Infof("Initializing Jaeger exporter via protocol %s", protocol)
		endpointOption = jaeger.WithAgentEndpoint()

	case "udp/thrift.compact":
		log.WithFields(logrus.Fields{
			"agentHost": os.Getenv("OTEL_EXPORTER_JAEGER_AGENT_HOST"),
			"agentPort": os.Getenv("OTEL_EXPORTER_JAEGER_AGENT_PORT"),
		}).Infof("Initializing Jaeger exporter via protocol %s", protocol)
		endpointOption = jaeger.WithAgentEndpoint()

	default:
		log.WithField("OTEL_EXPORTER_JAEGER_PROTOCOL", protocol).
			Error("Unknown OpenTelemetry protocol configured")
		endpointOption = jaeger.WithAgentEndpoint()
	}

	exp, err := jaeger.New(endpointOption)
	if err != nil {
		return nil, errors.Wrap(err, "error in jaeger.New")
	}

	return exp, nil
}
