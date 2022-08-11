# Distributed Tracing Using OpenTelemetry and Jaeger
## What is OpenTelemetry?
From [opentelemetry.io](https://opentelemetry.io):

> OpenTelemetry is a collection of tools, APIs, and SDKs. Use it to instrument,
> generate, collect, and export telemetry data (metrics, logs, and traces) to
> help you analyze your software’s performance and behavior.

Use OpenTelemetry to generate traces visualizing behavior in your application.
It's comprised of nested spans that are rendered as a waterfall graph.  Each
span indicates start/end timings and optionally other developer specified
metadata and logging output.

Jaeger Tracing is the tool used to receive OpenTelemetry trace data.  Use its
web UI to query for traces and view the waterfall graph.

OpenTelemetry is distributed, which allows services to pass the trace ids to
disparate remote services.  The remote service may generate child spans that
will be visible on the same waterfall graph.  This requires that all services
send traces to the same Jaeger Server.

## Why OpenTelemetry?
It is the latest standard for distributed tracing clients.

OpenTelemetry supersedes its now deprecated predecessor,
[OpenTracing](https://opentracing.io).

It no longer requires implementation specific client modules, such as Jaeger
client.  The provided OpenTelemetry SDK includes a client for Jaeger.

## Why Jaeger Tracing Server?
Easy to setup.  Powerful and easy to use web UI.  Open source.  Scalable using
Elasticsearch.

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

#### Export via UDP
By default, Jaeger exports to a Jaeger Agent on localhost port 6831/udp.  The
host and port can be changed by setting environment variables
`OTEL_EXPORTER_JAEGER_AGENT_HOST`, `OTEL_EXPORTER_JAEGER_AGENT_PORT`.

It's important to ensure UDP traces are sent on the loopback interface (aka
localhost).  UDP datagrams are limited in size to the MTU of the interface and
the payload cannot be split into multiple datagrams.  The loopback interface
MTU is large, typically 65000 or higher.  Network interface MTU is typically
much lower at 1500.  OpenTelemetry's Jaeger client is sometimes unable to limit
its payload to fit in a 1500 byte datagram and will drop those packets.  This
causes traces that are mangled or missing detail.

#### Export via HTTP
If it's not possible to install a Jaeger Agent on localhost, the client can
instead export directly to the Jaeger Collector of the Jaeger Server on HTTP
port 14268.

Enable HTTP exporter with configuration:
```
OTEL_EXPORTER_JAEGER_PROTOCOL=http/thrift.binary
OTEL_EXPORTER_JAEGER_ENDPOINT=http://<jaeger-server>:14268/api/traces
```

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
repo.

```go
import "github.com/mailgun/holster/v4/tracing"

ctx, tracer, err := tracing.InitTracing(ctx, "github.com/myrepo/myservice")

// ...

err = tracing.CloseTracing(context.Background())
```

### Log Level
Log level may be applied to traces to filter spans having a minimum log
severity.  Spans that do not meed the minimum severity are simply dropped and
not exported.

Log level is passed in `tracing.InitTracingWithLevel()` as a numeric
[RFC5424](https://www.rfc-editor.org/rfc/rfc5424) log level (0-7).

As a convenience, use log level constants from Logrus, like so:

```go
import "github.com/sirupsen/logrus"

_, _, err := tracing.InitTracingWithLevel(ctx, "my library name", int64(logrus.DebugLevel))
```

When using `tracing.InitTracing()`, the level will be the global level set in
Logrus.

See [Log Level in Scope](#log-level-in-scope) for details on creating spans
with assigned log level.

#### Log Level Filtering
Just like with common log frameworks, scope will filter spans that are a lower
severity than threshold provided to `InitTracingWithLevel()`.

If scopes are nested and one in the middle is dropped, the hierarchy will be
preserved.

e.g. If `InitTracingWithLevel()` is passed a log level of "Info", we expect
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

The tracer object identifies itself by a library name, which can be seen in Jaeger
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
	tracer := tracing.TracerFromContext(ctx)
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
```

### Scope Tracing
The scope functions automate span start/end and error reporting to the active
trace.

| Function       | Description |
| -------------- | ----------- |
| `StartScope()` | Start a scope by creating a span named after the fully qualified calling function. |
| `StartNamedScope()` | Start a scope by creating a span with user-provided name. |
| `EndScope()`   | End the scope, record returned error value. |
| `Scope()`      | Wraps a code block as a scope using `StartScope()`/`EndScope()` functionality. |
| `NamedScope()` | Same as `Scope()` with a user-provided span name. |

If the scope's action function returns an error, the error message is
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

#### Using `Scope()`
```go
import (
	"context"

	"github.com/mailgun/holster/v4/tracing"
	"github.com/sirupsen/logrus"
)

func MyFunc(ctx context.Context) error {
	return tracing.Scope(ctx, func(ctx context.Context) error {
		logrus.WithContext(ctx).Info("This message also logged to trace")

		// ...

		return nil
	})
}
```

#### Scope Log Level
Log level can be applied to individual spans using variants of
`Scope()`/`StartScope()` to set debug, info, warn, or error levels:

```go
ctx2 := tracing.StartScopeDebug(ctx)
defer tracing.EndScope(ctx2, nil)
```

```go
err := tracing.ScopeDebug(ctx, func(ctx context.Context) error {
    // ...

    return nil
})
```

#### Scope Log Level Filtering
Just like with common log frameworks, scope will filter spans that are a lower
severity than threshold provided to `InitTracingWithLevel()`.



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
	grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
	grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
)
```

#### gRPC Server
```go
import (
	"google.golang.org/grpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

grpcSrv := grpc.NewServer(
	grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
	grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
)
```

### Config Options
Possible environment config exporter config options when using `tracing.InitTracing()`.

#### Jaeger
* `OTEL_EXPORTER_JAEGER_PROTOCOL`
* `OTEL_EXPORTER_JAEGER_ENDPOINT`
* `OTEL_EXPORTER_JAEGER_AGENT_HOST`
* `OTEL_EXPORTER_JAEGER_AGENT_PORT`

#### `OTEL_EXPORTER_JAEGER_PROTOCOL`
  Possible values:
* `udp/thrift.compact` (default): Export traces via UDP datagrams.  Best used when Jaeger Agent is accessible via loopback interface.  May also provide `OTEL_EXPORTER_JAEGER_AGENT_HOST`/`OTEL_EXPORTER_JAEGER_AGENT_PORT`, which default to `localhost`/`6831`.
* `udp/thrift.binary`: Alternative protocol to the more commonly used `udp/thrift.compact`.  May also provide `OTEL_EXPORTER_JAEGER_AGENT_HOST`/`OTEL_EXPORTER_JAEGER_AGENT_PORT`, which default to `localhost`/`6832`.
* `http/thrift.compact`: Export traces via HTTP packets.  Best used when Jaeger Agent cannot be deployed or is inaccessible via loopback interface.  This setting sends traces directly to Jaeger's collector port.  May also provide `OTEL_EXPORTER_JAEGER_ENDPOINT`, which defaults to `http://localhost:14268/api/traces`.

#### HoneyComb
* `OTEL_EXPORTER_HONEYCOMB_ENDPOINT` - Defaults to `api.honeycomb.io:443`
* `OTEL_EXPORTER_HONEYCOMB_API_KEY`

Other environment config options recognized by OTel
libraries are defined [here](https://opentelemetry.io/docs/reference/specification/sdk-environment-variables/)


## Prometheus Metrics
Prometheus metric objects are defined at array `tracing.Metrics`.  Enable by registering these metrics in Prometheus.

| Metric | Description |
| ------ | ----------- |
| `holster_tracing_counter` | Count of traces generated by holster `tracing` package.  Label `error` contains `true` for traces in error status. |
| `holster_tracing_spans`   | Count of trace spans generated by holster `tracing` package.  Label `error` contains `true` for spans in error status. |
