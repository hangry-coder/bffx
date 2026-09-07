package cache

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type tracingProvider struct {
	underlying Provider
	tracer     trace.Tracer
}

// NewTracingProvider wraps a cache Provider with OpenTelemetry tracing instrumentation.
func NewTracingProvider(underlying Provider) Provider {
	if underlying == nil {
		return nil
	}
	return &tracingProvider{
		underlying: underlying,
		tracer:     otel.Tracer("bffx-cache"),
	}
}

func (tp *tracingProvider) Get(ctx context.Context, key string) ([]byte, error) {
	ctx, span := tp.tracer.Start(ctx, "Cache.Get", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.String("cache.provider", tp.underlying.Type()),
	)
	val, err := tp.underlying.Get(ctx, key)
	if err != nil && err.Error() != "cache miss" {
		span.RecordError(err)
	}
	span.SetAttributes(attribute.Bool("cache.hit", val != nil))
	return val, err
}

func (tp *tracingProvider) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ctx, span := tp.tracer.Start(ctx, "Cache.Set", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.String("cache.provider", tp.underlying.Type()),
		attribute.String("cache.ttl", ttl.String()),
	)
	err := tp.underlying.Set(ctx, key, value, ttl)
	if err != nil {
		span.RecordError(err)
	}
	return err
}

func (tp *tracingProvider) Delete(ctx context.Context, key string) error {
	ctx, span := tp.tracer.Start(ctx, "Cache.Delete", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.String("cache.provider", tp.underlying.Type()),
	)
	err := tp.underlying.Delete(ctx, key)
	if err != nil {
		span.RecordError(err)
	}
	return err
}

func (tp *tracingProvider) Flush(ctx context.Context) error {
	ctx, span := tp.tracer.Start(ctx, "Cache.Flush", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("cache.provider", tp.underlying.Type()),
	)
	err := tp.underlying.Flush(ctx)
	if err != nil {
		span.RecordError(err)
	}
	return err
}

func (tp *tracingProvider) Tags(tags ...string) Tagger {
	t := tp.underlying.Tags(tags...)
	return &tracingTagger{
		underlying: t,
		tracer:     tp.tracer,
		provider:   tp.underlying.Type(),
		tags:       tags,
	}
}

func (tp *tracingProvider) SetWithTags(ctx context.Context, key string, value []byte, tags []string, ttl time.Duration) error {
	ctx, span := tp.tracer.Start(ctx, "Cache.SetWithTags", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.String("cache.provider", tp.underlying.Type()),
		attribute.StringSlice("cache.tags", tags),
		attribute.String("cache.ttl", ttl.String()),
	)
	err := tp.underlying.SetWithTags(ctx, key, value, tags, ttl)
	if err != nil {
		span.RecordError(err)
	}
	return err
}

func (tp *tracingProvider) InvalidateTags(ctx context.Context, tags []string) (int, error) {
	ctx, span := tp.tracer.Start(ctx, "Cache.InvalidateTags", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.StringSlice("cache.tags", tags),
		attribute.String("cache.provider", tp.underlying.Type()),
	)
	n, err := tp.underlying.InvalidateTags(ctx, tags)
	if err != nil {
		span.RecordError(err)
	}
	span.SetAttributes(attribute.Int("cache.invalidated_count", n))
	return n, err
}

func (tp *tracingProvider) InvalidatePattern(ctx context.Context, pattern string) (int, error) {
	ctx, span := tp.tracer.Start(ctx, "Cache.InvalidatePattern", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("cache.pattern", pattern),
		attribute.String("cache.provider", tp.underlying.Type()),
	)
	n, err := tp.underlying.InvalidatePattern(ctx, pattern)
	if err != nil {
		span.RecordError(err)
	}
	span.SetAttributes(attribute.Int("cache.invalidated_count", n))
	return n, err
}

func (tp *tracingProvider) Type() string {
	return tp.underlying.Type()
}

func (tp *tracingProvider) Ping(ctx context.Context) error {
	ctx, span := tp.tracer.Start(ctx, "Cache.Ping", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	err := tp.underlying.Ping(ctx)
	if err != nil {
		span.RecordError(err)
	}
	return err
}

type tracingTagger struct {
	underlying Tagger
	tracer     trace.Tracer
	provider   string
	tags       []string
}

func (tt *tracingTagger) Get(ctx context.Context, key string) ([]byte, error) {
	ctx, span := tt.tracer.Start(ctx, "Cache.Tagger.Get", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.StringSlice("cache.tags", tt.tags),
		attribute.String("cache.provider", tt.provider),
	)
	val, err := tt.underlying.Get(ctx, key)
	if err != nil && err.Error() != "cache miss" {
		span.RecordError(err)
	}
	span.SetAttributes(attribute.Bool("cache.hit", val != nil))
	return val, err
}

func (tt *tracingTagger) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ctx, span := tt.tracer.Start(ctx, "Cache.Tagger.Set", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.StringSlice("cache.tags", tt.tags),
		attribute.String("cache.provider", tt.provider),
		attribute.String("cache.ttl", ttl.String()),
	)
	err := tt.underlying.Set(ctx, key, value, ttl)
	if err != nil {
		span.RecordError(err)
	}
	return err
}
