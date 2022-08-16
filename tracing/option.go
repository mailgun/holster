package tracing

import (
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type TracingOption interface {
	apply(state *initState)
}

type TracerProviderTracingOption struct {
	opts []sdktrace.TracerProviderOption
}

// WithTracerProviderOption passes TracerProviderOption arguments to
// InitTracing.
func WithTracerProviderOption(opts ...sdktrace.TracerProviderOption) *TracerProviderTracingOption {
	return &TracerProviderTracingOption{
		opts: opts,
	}
}

func (o *TracerProviderTracingOption) apply(state *initState) {
	state.opts = append(state.opts, o.opts...)
}

type ResourceOption struct {
	res *resource.Resource
}

// WithResource is convenience function for common use case of passing a
// Resource object as TracerProviderOption.
func WithResource(res *resource.Resource) *ResourceOption {
	return &ResourceOption{
		res: res,
	}
}

func (o *ResourceOption) apply(state *initState) {
	state.opts = append(state.opts, sdktrace.WithResource(o.res))
}

type LevelTracingOption struct {
	level Level
}

// WithLevel passes a log level to InitTracing.
func WithLevel(level Level) *LevelTracingOption {
	return &LevelTracingOption{
		level: level,
	}
}

func (o *LevelTracingOption) apply(state *initState) {
	state.level = o.level
}
