package storage

import (
	"context"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type tracingStore struct {
	underlying Store
	tracer     trace.Tracer
}

// NewTracingStore wraps a Store with OpenTelemetry tracing.
func NewTracingStore(underlying Store) Store {
	if underlying == nil {
		return nil
	}
	return &tracingStore{
		underlying: underlying,
		tracer:     otel.Tracer("bffx-storage"),
	}
}

func (ts *tracingStore) List(ctx context.Context, resource string, limit, offset int) ([]map[string]any, error) {
	ctx, span := ts.tracer.Start(ctx, "Storage.List", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "sql"),
		attribute.String("db.operation", "list"),
		attribute.String("db.resource", resource),
		attribute.Int("db.limit", limit),
		attribute.Int("db.offset", offset),
	)
	res, err := ts.underlying.List(ctx, resource, limit, offset)
	if err != nil {
		span.RecordError(err)
	}
	return res, err
}

func (ts *tracingStore) Get(ctx context.Context, resource, id string) (map[string]any, error) {
	ctx, span := ts.tracer.Start(ctx, "Storage.Get", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "sql"),
		attribute.String("db.operation", "get"),
		attribute.String("db.resource", resource),
		attribute.String("db.id", id),
	)
	res, err := ts.underlying.Get(ctx, resource, id)
	if err != nil {
		span.RecordError(err)
	}
	return res, err
}

func (ts *tracingStore) GetByField(ctx context.Context, resource, field, value string) (map[string]any, error) {
	ctx, span := ts.tracer.Start(ctx, "Storage.GetByField", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "sql"),
		attribute.String("db.operation", "get_by_field"),
		attribute.String("db.resource", resource),
		attribute.String("db.field", field),
	)
	res, err := ts.underlying.GetByField(ctx, resource, field, value)
	if err != nil {
		span.RecordError(err)
	}
	return res, err
}

func (ts *tracingStore) Create(ctx context.Context, resource string, payload map[string]any) (map[string]any, error) {
	ctx, span := ts.tracer.Start(ctx, "Storage.Create", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "sql"),
		attribute.String("db.operation", "create"),
		attribute.String("db.resource", resource),
	)
	res, err := ts.underlying.Create(ctx, resource, payload)
	if err != nil {
		span.RecordError(err)
	}
	return res, err
}

func (ts *tracingStore) Update(ctx context.Context, resource, id string, payload map[string]any) (map[string]any, error) {
	ctx, span := ts.tracer.Start(ctx, "Storage.Update", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "sql"),
		attribute.String("db.operation", "update"),
		attribute.String("db.resource", resource),
		attribute.String("db.id", id),
	)
	res, err := ts.underlying.Update(ctx, resource, id, payload)
	if err != nil {
		span.RecordError(err)
	}
	return res, err
}

func (ts *tracingStore) Delete(ctx context.Context, resource, id string) error {
	ctx, span := ts.tracer.Start(ctx, "Storage.Delete", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "sql"),
		attribute.String("db.operation", "delete"),
		attribute.String("db.resource", resource),
		attribute.String("db.id", id),
	)
	err := ts.underlying.Delete(ctx, resource, id)
	if err != nil {
		span.RecordError(err)
	}
	return err
}

func (ts *tracingStore) ListByOwner(ctx context.Context, resource, ownerID string, limit, offset int) ([]map[string]any, error) {
	ctx, span := ts.tracer.Start(ctx, "Storage.ListByOwner", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "sql"),
		attribute.String("db.operation", "list_by_owner"),
		attribute.String("db.resource", resource),
		attribute.String("db.owner_id", ownerID),
		attribute.Int("db.limit", limit),
		attribute.Int("db.offset", offset),
	)
	res, err := ts.underlying.ListByOwner(ctx, resource, ownerID, limit, offset)
	if err != nil {
		span.RecordError(err)
	}
	return res, err
}

func (ts *tracingStore) Reconcile(ctx context.Context, reg *manifest.Registry) ([]Change, error) {
	ctx, span := ts.tracer.Start(ctx, "Storage.Reconcile", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	res, err := ts.underlying.Reconcile(ctx, reg)
	if err != nil {
		span.RecordError(err)
	}
	return res, err
}

func (ts *tracingStore) GetChildren(ctx context.Context, resource, id string) ([]map[string]any, error) {
	ctx, span := ts.tracer.Start(ctx, "Storage.GetChildren", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "sql"),
		attribute.String("db.operation", "get_children"),
		attribute.String("db.resource", resource),
		attribute.String("db.id", id),
	)
	res, err := ts.underlying.GetChildren(ctx, resource, id)
	if err != nil {
		span.RecordError(err)
	}
	return res, err
}

func (ts *tracingStore) GetAncestors(ctx context.Context, resource, id string) ([]map[string]any, error) {
	ctx, span := ts.tracer.Start(ctx, "Storage.GetAncestors", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "sql"),
		attribute.String("db.operation", "get_ancestors"),
		attribute.String("db.resource", resource),
		attribute.String("db.id", id),
	)
	res, err := ts.underlying.GetAncestors(ctx, resource, id)
	if err != nil {
		span.RecordError(err)
	}
	return res, err
}

func (ts *tracingStore) Query(ctx context.Context, resource string) QueryBuilder {
	qb := ts.underlying.Query(ctx, resource)
	return &tracingQueryBuilder{
		underlying: qb,
		tracer:     ts.tracer,
		resource:   resource,
	}
}

type tracingQueryBuilder struct {
	underlying QueryBuilder
	tracer     trace.Tracer
	resource   string
}

func (tqb *tracingQueryBuilder) Where(field, op string, value any) QueryBuilder {
	tqb.underlying = tqb.underlying.Where(field, op, value)
	return tqb
}

func (tqb *tracingQueryBuilder) WhereIn(field string, values []any) QueryBuilder {
	tqb.underlying = tqb.underlying.WhereIn(field, values)
	return tqb
}

func (tqb *tracingQueryBuilder) OrderBy(field string, desc bool) QueryBuilder {
	tqb.underlying = tqb.underlying.OrderBy(field, desc)
	return tqb
}

func (tqb *tracingQueryBuilder) Limit(n int) QueryBuilder {
	tqb.underlying = tqb.underlying.Limit(n)
	return tqb
}

func (tqb *tracingQueryBuilder) Offset(n int) QueryBuilder {
	tqb.underlying = tqb.underlying.Offset(n)
	return tqb
}

func (tqb *tracingQueryBuilder) Execute(ctx context.Context) ([]map[string]any, error) {
	ctx, span := tqb.tracer.Start(ctx, "Storage.Query.Execute", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "sql"),
		attribute.String("db.operation", "query"),
		attribute.String("db.resource", tqb.resource),
	)
	res, err := tqb.underlying.Execute(ctx)
	if err != nil {
		span.RecordError(err)
	}
	return res, err
}

func (tqb *tracingQueryBuilder) Count(ctx context.Context) (int, error) {
	ctx, span := tqb.tracer.Start(ctx, "Storage.Query.Count", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "sql"),
		attribute.String("db.operation", "count"),
		attribute.String("db.resource", tqb.resource),
	)
	res, err := tqb.underlying.Count(ctx)
	if err != nil {
		span.RecordError(err)
	}
	return res, err
}
