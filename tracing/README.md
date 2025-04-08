# Distributed Tracing Using OpenTelemetry
## What is OpenTelemetry?
From [opentelemetry.io](https://opentelemetry.io):

> OpenTelemetry is a collection of tools, APIs, and SDKs. Use it to instrument,
> generate, collect, and export telemetry data (metrics, logs, and traces) to
> help you analyze your software’s performance and behavior.

Use OpenTelemetry to generate traces visualizing behavior in your application.
It's comprised of nested spans that are rendered as a waterfall graph.  Each
span indicates start/end timings and optionally other developer specified
metadata and logging output.

OpenTelemetry is distributed, which allows services to pass the trace ids to
disparate remote services.  The remote service may generate child spans that
will be visible on the same waterfall graph.  This requires that all services
send traces to the same tracing server.

## Why OpenTelemetry?
It is the latest standard for distributed tracing clients.

OpenTelemetry supersedes its now deprecated predecessor,
[OpenTracing](https://opentracing.io).

The OpenTelmetry SDK provides client packages that are used to export traces
over standard OTLP protocol.

## Why Jaeger Tracing Server?
Easy to setup.  Powerful and easy to use web UI.  Open source.  Scalable using
Elasticsearch persistence if you outgrow the default in-memory storage.

Jaeger Tracing Server is a common tool used to receive OpenTelemetry trace
data.  Use its web UI to query for traces and view the waterfall graph.  For
the sake of simplicity, all examples in this document assume using Jaeger
Server for collecting and reporting on traces.  Though, there are other free
and commercial tools that serve the same function.

## Getting Started
[opentelemetry.io](https://opentelemetry.io)

OpenTelemetry dev reference:
[https://pkg.go.dev/go.opentelemetry.io/otel](https://pkg.go.dev/go.opentelemetry.io/otel)

See unit tests for usage examples.

### Configuration
In ideal conditions where you wish to send traces to localhost on 6831/udp, no
configuration is necessary.

Configuration reference via environment variables:
[https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/sdk-environment-variables.md](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/sdk-environment-variables.md).

#### Exporter
Traces export to OTLP by default.  Other exporters are available by setting
environment variable `OTEL_TRACES_EXPORTER` to:

* `otlp`: [OTLP: OpenTelemetry Protocol](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md)
* `none`: Disable export.

Note: OpenTelemetry [dropped support for Jaeger
client](https://github.com/open-telemetry/opentelemetry-go/blob/9b06c4cd35edefa3ff7308e8f074d9bc34168e13/CHANGELOG.md?plain=1#L770-L773).
The Jaeger protocol is no longer required because the Jaeger Server now
supports OTLP protocol for inbound traces.  As a result, Holster no longer supports a `jaeger` exporter setting.

#### OTLP Exporter
By default, OTLP exporter exports to an [OpenTelemetry
Collector](https://opentelemetry.io/docs/collector/) on localhost on gRPC port
4317.  The host and port can be changed by setting environment variable
`OTEL_EXPORTER_OTLP_ENDPOINT` like `https://collector:4317`.

See more: [OTLP configuration](#OTLP)

#### Probabilistic Sampling
By default, all traces are sampled.  If the tracing volume is burdening the
application, network, or Jaeger Server, then sampling can be used to
selectively drop some of the traces.

In production, it may be ideal to set sampling based on a percentage
probability.  The probability can be set in Jaeger Server configuration or
locally.

To enable locally, set environment variables:

```
OTEL_TRACES_SAMPLER=traceidratio
OTEL_TRACES_SAMPLER_ARG=<value-between-0-and-1>
```

Where 1 is always sample every trace and 0 is do not sample anything.

### Initialization
The OpenTelemetry client must be initialized to read configuration and prepare
a `Tracer` object.  When application is exiting, call `CloseTracing()`.

The library name passed in the second argument appears in spans as metadata
`otel.library.name`.  This is used to identify the library or module that
generated that span.  This usually the fully qualified module name of your
repo.  Pass an empty string to autodetect the module name of the executible.

```go
import "github.com/mailgun/holster/v4/tracing"

err := tracing.InitTracing(ctx, "github.com/myrepo/myservice")
// or...
err := tracing.InitTracing(ctx, "")

// ...

err = tracing.CloseTracing(ctx)
```

### Log Level
Log level may be applied to traces to filter spans having a minimum log
severity.  Spans that do not meed the minimum severity are simply dropped and
not exported.

Log level is passed with option `tracing.WithLevel()` as a numeric
log level (0-6): Panic, Fatal, Error, Warning, Info, Debug, Trace.

As a convenience, use constants, such as `tracing.DebugLevel`:

```go
import "github.com/mailgun/holster/v4/tracing"

level := tracing.DebugLevel
err := tracing.InitTracing(ctx, "my library name", tracing.WithLevel(level))
```

If `WithLevel()` is omitted, the level will be the global level set in Logrus.

See [Scope Log Level](#scope-log-level) for details on creating spans
with an assigned log level.

#### Log Level Filtering
Just like with common log frameworks, scope will filter spans that are a lower
severity than threshold provided using `WithLevel()`.

If scopes are nested and one in the middle is dropped, the hierarchy will be
preserved.

e.g. If `WithLevel()` is passed a log level of "Info", we expect
"Debug" scopes to be dropped:
```
# Input:
Info Level 1 -> Debug Level 2 -> Info Level 3

# Exports spans in form:
Info Level 1 -> Info Level 3
```

Log level filtering is critical for high volume applications where debug
tracing would generate significantly more data that isn't sustainable or
helpful for normal operations.  But developers will have the option to
selectively enable debug tracing for troubleshooting.

### Tracer Lifecycle
The common use case is to call `InitTracing()` to build a single default tracer
that the application uses througout its lifetime, then call `CloseTracing()` on
shutdown.

The default tracer is stored globally in the tracer package for use by tracing
functions.

The tracer object identifies itself by a library name, which can be seen in
traces as attribute `otel.library.name`.  This value is typically the module
name of the application.

If it's necessary to create traces with a different library name, additional
tracer objects may be created by `NewTracer()` which returns a context with the
tracer object embedded in it.  This context object must be passed to tracing
functions use that tracer in particular, otherwise the default tracer will be
selected.

### Setting Resources
OpenTelemetry is configured by environment variables and supplemental resource
settings.  Some of these resources also map to environment variables.

#### Service Name
The service name appears in the Jaeger "Service" dropdown.  If unset, default
is `unknown_service:<executable-filename>`.

Service name may be set in configuration by environment variable
`OTEL_SERVICE_NAME`.

As an alternative to environment variable, it may be provided as a resource.
The resource setting takes precedent over the environment variable.

```go
import "github.com/mailgun/holster/v4/tracing"

res, err := tracing.NewResource("My service", "v1.0.0")
ctx, tracer, err := tracing.InitTracing(ctx, "github.com/myrepo/myservice", tracing.WithResource(res))
```

### Manual Tracing
Basic instrumentation.  Traces function duration as a span and captures logrus logs.

```go
import (
	"context"

	"github.com/mailgun/holster/v4/tracing"
)

func MyFunc(ctx context.Context) error {
	tracer := tracing.Tracer()
	ctx, span := tracer.Start(ctx, "Span name")
	defer span.End()

	// ...

	return nil
}
```

### Common OpenTelemetry Tasks
#### Span Attributes
The active `Span` object is embedded in the `Context` object.  This can be
extracted to do things like add attribute metadata to the span:

```go
import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func MyFunc(ctx context.Context) error {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("foobar", "value"),
		attribute.Int("x", 12345),
	)
}
```

#### Add Span Event
A span event is a log message added to the active span.  It can optionally
include attribute metadata.

```go
span.AddEvent("My message")
span.AddEvent("My metadata", trace.WithAttributes(
	attribute.String("foobar", "value"),
	attribute.Int("x", 12345"),
))
```

#### Log an Error
An `Error` object can be logged to the active span.  This appears as a log
event on the span.

```go
err := errors.New("My error message")
span.RecordError(err)

// Can also add attribute metadata.
span.RecordError(err, trace.WithAttributes(
	attribute.String("foobar", "value"),
))
```

### Scope Tracing
The scope functions automate span start/end and error reporting to the active
trace.

| Function       | Description |
| -------------- | ----------- |
| `StartScope()`/`BranchScope()` | Start a scope by creating a span named after the fully qualified calling function. |
| `StartNamedScope()`/`BranchNamedScope()` | Start a scope by creating a span with user-provided name. |
| `EndScope()`   | End the scope, record returned error value. |
| `CallScope()`/`CallScopeBranch()` | Call a code block as a scope using `StartScope()`/`EndScope()` functionality. |
| `CallNamedScope()`/`CallNamedScopeBranch()` | Same as `CallScope()` with a user-provided span name. |

The secondary `Branch` functions perform the same task as their counterparts,
except that it will "branch" from an existing trace only.  If the context
contains no trace id, no trace will be created.  The `Branch` functions are
best used with lower level or shared code where there is no value in creating a
trace starting at that point.

If the `CallScope()` action function returns an error, the error message is
automatically logged to the trace and the trace is marked as error.

#### Using `StartScope()`/`EndScope()`
```go
import (
	"context"

	"github.com/mailgun/holster/tracing"
	"github.com/sirupsen/logrus"
)

func MyFunc(ctx context.Context) (reterr error) {
	ctx = tracing.StartScope(ctx)
	defer func() {
		tracing.EndScope(ctx, reterr)
	}()

	logrus.WithContext(ctx).Info("This message also logged to trace")

	// ...

	return nil
}
```

#### Using `CallScope()`
```go
import (
	"context"

	"github.com/mailgun/holster/v4/tracing"
	"github.com/sirupsen/logrus"
)

func MyFunc(ctx context.Context) error {
	return tracing.CallScope(ctx, func(ctx context.Context) error {
		logrus.WithContext(ctx).Info("This message also logged to trace")

		// ...

		return nil
	})
}
```

#### Scope Log Level
Log level can be applied to individual spans using variants of
`CallScope()`/`StartScope()` to set debug, info, warn, or error levels:

```go
ctx2 := tracing.StartScopeDebug(ctx)
defer tracing.EndScope(ctx2, nil)
```

```go
err := tracing.CallScopeDebug(ctx, func(ctx context.Context) error {
    // ...

    return nil
})
```

#### Scope Log Level Filtering
Just like with common log frameworks, scope will filter spans that are a lower
severity than threshold provided using `WithLevel()`.


## Instrumentation
### Logrus
Logrus is configured by `InitTracing()` to mirror log messages to the active trace, if exists.

For this to work, you must use the `WithContext()` method to propagate the active
trace stored in the context.

```go
logrus.WithContext(ctx).Info("This message also logged to trace")
```

If the log is error level or higher, the span is also marked as error and sets
attributes `otel.status_code` and `otel.status_description` with the error
details.

### Other Instrumentation Options
See: [https://opentelemetry.io/registry/?language=go&component=instrumentation](https://opentelemetry.io/registry/?language=go&component=instrumentation)

#### gRPC Client
Client's trace ids are propagated to the server.  A span will be created for
the client call and another one for the server side.

```go
import (
	"google.golang.org/grpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

conn, err := grpc.Dial(server,
	grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
)
```

#### gRPC Server
```go
import (
	"google.golang.org/grpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

grpcSrv := grpc.NewServer(
	grpc.StatsHandler(otelgrpc.NewServerHandler()),
)
```

### Config Options
Possible environment config exporter config options when using
`tracing.InitTracing()`.

#### OTLP
* `OTEL_EXPORTER_OTLP_PROTOCOL`
   * May be one of: `grpc`, `http/protobuf`.
* `OTEL_EXPORTER_OTLP_ENDPOINT`
   * Set to URL like `http://collector:<port>` or `https://collector:<port>`.
   * Port for `grpc` protocol is 4317, `http/protobuf` is 4318.
   * If protocol is `grpc`, URL scheme `http` indicates insecure TLS
     connection, `https` indicates secure even though connection is over gRPC
     protocol, not HTTP(S).
* `OTEL_EXPORTER_OTLP_CERTIFICATE`, `OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE`,
  `OTEL_EXPORTER_OTLP_CLIENT_KEY`
   * If protocol is `grpc` or using HTTPS endpoint, set TLS certificate files.
* `OTEL_EXPORTER_OTLP_HEADERS`
   * Optional headers passed to collector in format:
     `key=value,key2=value2,...`.

See also [OTLP configuration
reference](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/exporter.md).

#### Honeycomb
[Honeycomb](https://honeycomb.io) consumes OTLP traces and requires an API key header:
```
OTEL_EXPORTER_OTLP_PROTOCOL=otlp
OTEL_EXPORTER_OTLP_ENDPOINT=https://api.honeycomb.io
OTEL_EXPORTER_OTLP_HEADERS=x-honeycomb-team=<api-key>
```


## Prometheus Metrics
Prometheus metric objects are defined at array `tracing.Metrics`.  Enable by registering these metrics in Prometheus.

| Metric | Description |
| ------ | ----------- |
| `holster_tracing_counter` | Count of traces generated by holster `tracing` package.  Label `error` contains `true` for traces in error status. |
| `holster_tracing_spans`   | Count of trace spans generated by holster `tracing` package.  Label `error` contains `true` for spans in error status. |
