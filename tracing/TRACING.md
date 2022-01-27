# Distributed Tracing Using OpenTelemetry and Jaeger
## What is OpenTelemetry?
From [opentelemetry.io](https://opentelemetry.io):

> OpenTelemetry is a collection of tools, APIs, and SDKs. Use it to instrument,
> generate, collect, and export telemetry data (metrics, logs, and traces) to
> help you analyze your software’s performance and behavior.

Use OpenTelemetry to generate traces visualizing bnehavior in your application.
It's comprised of nested spans that are visualized as a waterfall graph.  Each
span indicates start/end timings and optionally other developer specified
metadata and logging output.

Jaeger Tracing is the tool used to receive OpenTelemetry trace data.  Use its
web UI to query for traces and view the waterfall graph.

OpenTelemetry is distributed, which allows you to pass the trace ids to
disparate remote services.  The remote service may generate child spans that
will be visible on the same waterfall graph.  This requires that all services
send traces to the same Jaeger server.

## Why OpenTelemetry?
It is the latest standard for distributed tracing clients.

OpenTelemetry supercedes its now deprecated predecessor,
[OpenTracing](https://opentracing.io).

It no longer requires implementation specific client modules, such as Jaeger
client.  The provided OpenTelemetry SDK includes a client for Jaeger.

## Why Jaeger Tracing server?
Easy to setup.  Powerful and easy to use web UI.  Open source.

Traces are sent as UDP packets.  This minimizes burden on client to not
need to maintain an open socket or rely on server response time.  If tracing is
not needed, the packets sent by the client will be simply discarded by the OS.

## Getting Started
[opentelemetry.io](https://opentelemetry.io)

OpenTelemetry dev reference:
[https://pkg.go.dev/go.opentelemetry.io/otel](https://pkg.go.dev/go.opentelemetry.io/otel)

### Configuration
Configuration via environment variables:
[https://github.com/open-telemetry/opentelemetry-go/tree/main/exporters/jaeger](https://github.com/open-telemetry/opentelemetry-go/tree/main/exporters/jaeger)
Such as:
```
OTEL_EXPORTER_JAEGER_AGENT_HOST=<hostname|ip>
```

### Initialization
The OpenTelemetry client must be initialized to read configuration and prepare
a `Tracer` object.  When application is exiting, call `CloseTracing()`.

```go
import "github.com/mailgun/holster/tracing"

ctx, tracer, err := tracing.InitTracing(ctx, "My service")

// ...

tracing.CloseTracing(ctx)
```

### Manual Tracing
```go
import (
	"context"

	"github.com/mailgun/holster/tracing"
)

func MyFunc(ctx context.Context) error {
	tracer := tracing.TracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "Span name")
	defer span.End()

	// ...

	return nil
}
```

### Scope Tracing
The `Scope()` function automates span start/end, error reporting, and logging
to the active trace.

```go
import (
	"context"

	"github.com/mailgun/holster/tracing"
	"github.com/sirupsen/logrus"
)

func MyFunc(ctx context.Context) error {
	return tracing.Scope(ctx, "", func(ctx context.Context) error {
		logrus.WithContext(ctx).Info("This message also logged to trace")

		// ...

		return nil
	})
}
```

## Instrumentation
### Logrus
Logrus is configured by `InitTracing()` to mirror log messages to the active trace, if exists.

For this to work, you must use the `WithContext()` method to propagate the active
trace stored in the context.

```go
logrus.WithContext(ctx).Info("This message also logged to trace")
```

If the log is error level or higher, the span is marked as error and sets attributes
`otel.status_code` and `otel.status_description` with the error details.

### Other Instrumentation Options
[https://opentelemetry.io/registry/?language=go&component=instrumentation](https://opentelemetry.io/registry/?language=go&component=instrumentation)
