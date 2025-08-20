package refstore

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func StoreWithOtel(store Store) Store {
	return &WithOtel{
		tracer: otel.Tracer(name),
		Store:  store,
	}
}

var _ Store = (*WithOtel)(nil)

type WithOtel struct {
	tracer trace.Tracer
	Store  Store
}

// AddDependency implements Store.
func (w *WithOtel) AddDependency(ctx context.Context, ref string, dependency string) error {
	_, span := w.tracer.Start(
		ctx,
		"RefStore.AddDependency",
		trace.WithAttributes(attribute.String("ref", ref), attribute.String("dependency", dependency)),
	)
	defer span.End()

	return w.Store.AddDependency(ctx, ref, dependency)
}

// Close implements Store.
func (w *WithOtel) Close() error {
	return w.Store.Close()
}

// CommitTransaction implements Store.
func (w *WithOtel) CommitTransaction(ctx context.Context, message string) error {
	_, span := w.tracer.Start(
		ctx,
		"RefStore.CommitTransaction",
		trace.WithAttributes(attribute.String("message", message)),
	)
	defer span.End()

	return w.Store.CommitTransaction(ctx, message)
}

// Delete implements Store.
func (w *WithOtel) Delete(ctx context.Context, ref string) error {
	_, span := w.tracer.Start(
		ctx,
		"RefStore.Delete",
		trace.WithAttributes(attribute.String("ref", ref)),
	)
	defer span.End()

	return w.Store.Delete(ctx, ref)
}

// Get implements Store.
func (w *WithOtel) Get(ctx context.Context, ref string, v any) error {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent("RefStore.Get", trace.WithAttributes(attribute.String("ref", ref)))
	}

	return w.Store.Get(ctx, ref, v)
}

// GetDependants implements Store.
func (w *WithOtel) GetDependants(ctx context.Context, ref string) ([]string, error) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent("RefStore.GetDependants", trace.WithAttributes(attribute.String("ref", ref)))
	}

	return w.Store.GetDependants(ctx, ref)
}

// GetDependencies implements Store.
func (w *WithOtel) GetDependencies(ctx context.Context, ref string) ([]string, error) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent("RefStore.GetDependencies", trace.WithAttributes(attribute.String("ref", ref)))
	}

	return w.Store.GetDependencies(ctx, ref)
}

// GetLinks implements Store.
func (w *WithOtel) GetLinks(ctx context.Context, ref string) ([]string, error) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent("RefStore.GetLinks", trace.WithAttributes(attribute.String("ref", ref)))
	}

	return w.Store.GetLinks(ctx, ref)
}

// Link implements Store.
func (w *WithOtel) Link(ctx context.Context, ref string, target string) error {
	_, span := w.tracer.Start(
		ctx,
		"RefStore.Link",
		trace.WithAttributes(attribute.String("ref", ref), attribute.String("target", target)),
	)
	defer span.End()

	return w.Store.Link(ctx, ref, target)
}

// Match implements Store.
func (w *WithOtel) Match(ctx context.Context, glob ...string) ([]string, error) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent("RefStore.Match", trace.WithAttributes(attribute.String("glob", strings.Join(glob, ","))))
	}

	return w.Store.Match(ctx, glob...)
}

// MatchOptions implements Store.
func (w *WithOtel) MatchOptions(ctx context.Context, options MatchOptions, glob ...string) ([]string, error) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent("RefStore.MatchOptions", trace.WithAttributes(attribute.String("glob", strings.Join(glob, ","))))
	}

	return w.Store.MatchOptions(ctx, options, glob...)
}

// RemoveDependency implements Store.
func (w *WithOtel) RemoveDependency(ctx context.Context, ref string, dependency string) error {
	_, span := w.tracer.Start(
		ctx,
		"RefStore.RemoveDependency",
		trace.WithAttributes(attribute.String("ref", ref), attribute.String("dependency", dependency)),
	)
	defer span.End()

	return w.Store.RemoveDependency(ctx, ref, dependency)
}

// ResolveLink implements Store.
func (w *WithOtel) ResolveLink(ctx context.Context, ref string) (string, error) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent("RefStore.ResolveLink", trace.WithAttributes(attribute.String("ref", ref)))
	}

	return w.Store.ResolveLink(ctx, ref)
}

// Set implements Store.
func (w *WithOtel) Set(ctx context.Context, ref string, v any) error {
	_, span := w.tracer.Start(
		ctx,
		"RefStore.Set",
		trace.WithAttributes(attribute.String("ref", ref)),
	)
	defer span.End()

	return w.Store.Set(ctx, ref, v)
}

// StartTransaction implements Store.
func (w *WithOtel) StartTransaction(ctx context.Context) error {
	_, span := w.tracer.Start(
		ctx,
		"RefStore.StartTransaction",
	)
	defer span.End()

	return w.Store.StartTransaction(ctx)
}

// Unlink implements Store.
func (w *WithOtel) Unlink(ctx context.Context, ref string) error {
	_, span := w.tracer.Start(
		ctx,
		"RefStore.Unlink",
	)
	defer span.End()

	return w.Store.Unlink(ctx, ref)
}
