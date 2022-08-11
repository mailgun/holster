package tracing

import (
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
	res sdktrace.Resource
}

// WithResource is convenience function for common use case of passing a
// Resource object as TracerProviderOption.
func WithResource(res sdktrace.Resource) {
	return &ResourceOption{
		res: res,
	}
}

func (o *ResourceOption) apply(state *initState) {
	state.opts = append(state.opts, sdktrace.WithResource(res))
}

type LevelTracingOption struct {
	level int64
}

// WithLevel passes a log level to InitTracing.
// `level` is RFC5424 numeric log level (0-7).
// For convenience, use logrus constants, such as: `int64(logrus.InfoLevel)`
func WithLevel(level int64) *LevelTracingOption {
	return &LevelTracingOption{
		level: level,
	}
}

func (o *LevelTracingOption) apply(state *initState) {
	state.level = o.level
}
