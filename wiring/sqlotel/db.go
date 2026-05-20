// Package sqlotel decorates gitlab.com/gorib/storage/sql.Db with
// OpenTelemetry tracing AND metrics without modifying the base package.
//
// Usage in a service's DI config:
//
//	di.Wire[sql.Db](sqlotel.NewDb, di.Defaults(map[int]any{
//	    0: env.NeedValue[string]("DB_DRIVER"),
//	    1: env.NeedValue[string]("DB_DSN"),
//	}))
//
// TracerProvider and MeterProvider are resolved by DI from tools/otel.
// With noop providers the decorator's overhead is one method call per query.
//
// Cardinality: db.operation is bounded ({begin,query,queryRow,exec,commit,
// rollback}), status is bounded ({ok,error}). The raw db.statement appears
// only as a SPAN attribute, never as a metric label.
package sqlotel

import (
	"context"
	dbSql "database/sql"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"gitlab.com/gorib/storage/sql"
)

const instrumentationName = "gitlab.com/gorib/storage/sql"

func NewDb(driver, dsn string, tp trace.TracerProvider, mp metric.MeterProvider) (sql.Db, error) {
	inner, err := sql.NewDb(driver, dsn)
	if err != nil {
		return nil, err
	}
	meter := mp.Meter(instrumentationName)
	duration, _ := meter.Float64Histogram(
		"db.client.operation.duration",
		metric.WithDescription("Duration of a database operation"),
		metric.WithUnit("s"),
	)
	operations, _ := meter.Int64Counter(
		"db.client.operations",
		metric.WithDescription("Total database operations issued"),
	)
	return &tracedDb{
		inner: inner,
		instruments: instruments{
			tracer:     tp.Tracer(instrumentationName),
			duration:   duration,
			operations: operations,
		},
	}, nil
}

type instruments struct {
	tracer     trace.Tracer
	duration   metric.Float64Histogram
	operations metric.Int64Counter
}

func (i instruments) record(ctx context.Context, op string, took float64, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	attrs := metric.WithAttributes(
		attribute.String("db.operation", op),
		attribute.String("status", status),
	)
	i.duration.Record(ctx, took, attrs)
	i.operations.Add(ctx, 1, attrs)
}

type tracedDb struct {
	inner sql.Db
	instruments
}

func (t *tracedDb) Connect(ctx context.Context) (sql.Db, error) {
	conn, err := t.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &tracedDb{inner: conn, instruments: t.instruments}, nil
}

func (t *tracedDb) Begin(ctx context.Context) (sql.Tx, error) {
	ctx, span := t.tracer.Start(ctx, "sql.begin", trace.WithSpanKind(trace.SpanKindClient))
	start := time.Now()
	tx, err := t.inner.Begin(ctx)
	took := time.Since(start).Seconds()
	if err != nil {
		recordErr(span, err)
		span.End()
		t.record(ctx, "begin", took, err)
		return nil, err
	}
	t.record(ctx, "begin", took, nil)
	return &tracedTx{inner: tx, instruments: t.instruments, span: span}, nil
}

func (t *tracedDb) Driver() sql.Driver        { return t.inner.Driver() }
func (t *tracedDb) DbSql() (*dbSql.DB, error) { return t.inner.DbSql() }

type tracedTx struct {
	inner sql.Tx
	instruments
	span trace.Span // outer transaction span
}

func (t *tracedTx) Query(ctx context.Context, query string, args ...any) (sql.Rows, error) {
	ctx, span := t.startQuerySpan(ctx, "sql.query", query)
	defer span.End()
	start := time.Now()
	rows, err := t.inner.Query(ctx, query, args...)
	recordErr(span, err)
	t.record(ctx, "query", time.Since(start).Seconds(), err)
	return rows, err
}

func (t *tracedTx) QueryRow(ctx context.Context, query string, args ...any) sql.Row {
	ctx, span := t.startQuerySpan(ctx, "sql.queryRow", query)
	defer span.End()
	start := time.Now()
	row := t.inner.QueryRow(ctx, query, args...)
	t.record(ctx, "queryRow", time.Since(start).Seconds(), nil)
	return row
}

func (t *tracedTx) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	ctx, span := t.startQuerySpan(ctx, "sql.exec", query)
	defer span.End()
	start := time.Now()
	res, err := t.inner.Exec(ctx, query, args...)
	recordErr(span, err)
	t.record(ctx, "exec", time.Since(start).Seconds(), err)
	return res, err
}

func (t *tracedTx) Commit(ctx context.Context) error {
	start := time.Now()
	err := t.inner.Commit(ctx)
	recordErr(t.span, err)
	t.span.End()
	t.record(ctx, "commit", time.Since(start).Seconds(), err)
	return err
}

func (t *tracedTx) Rollback(ctx context.Context) error {
	start := time.Now()
	err := t.inner.Rollback(ctx)
	recordErr(t.span, err)
	t.span.End()
	t.record(ctx, "rollback", time.Since(start).Seconds(), err)
	return err
}

func (t *tracedTx) startQuerySpan(ctx context.Context, name, query string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("db.statement", query)),
	)
}

func recordErr(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
