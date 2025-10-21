package refstore

import (
	"context"
	"os"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var DEBUG_TRACES = os.Getenv("OCUROOT_DEBUG_TRACES") != ""

func StoreWithOtel(store Store) Store {
	if DEBUG_TRACES {
		return &WithOtel{
			Store: store,
		}
	}
	return store
}

var _ Store = (*WithOtel)(nil)

type WithOtel struct {
	Store              Store
	transactionMessage string
}

func (w *WithOtel) Info() StoreInfo {
	return w.Store.Info()
}

// Close implements Store.
func (w *WithOtel) Close() error {
	return w.Store.Close()
}

// CommitTransaction implements Store.
func (w *WithOtel) CommitTransaction(ctx context.Context) error {
	_, span := tracer.Start(
		ctx,
		"RefStore.CommitTransaction",
		trace.WithAttributes(attribute.String("message", w.transactionMessage)),
	)
	defer span.End()

	w.transactionMessage = ""
	return w.Store.CommitTransaction(ctx)
}

// Delete implements Store.
func (w *WithOtel) Delete(ctx context.Context, ref string) error {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent("RefStore.Delete", trace.WithAttributes(attribute.String("ref", ref)))
	}

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
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent("RefStore.Link", trace.WithAttributes(attribute.String("ref", ref), attribute.String("target", target)))
	}

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
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent("RefStore.Set", trace.WithAttributes(attribute.String("ref", ref)))
	}

	return w.Store.Set(ctx, ref, v)
}

// StartTransaction implements Store.
func (w *WithOtel) StartTransaction(ctx context.Context, message string) error {
	_, span := tracer.Start(
		ctx,
		"RefStore.StartTransaction",
		trace.WithAttributes(attribute.String("message", message)),
	)
	defer span.End()

	w.transactionMessage = message
	return w.Store.StartTransaction(ctx, message)
}

// Unlink implements Store.
func (w *WithOtel) Unlink(ctx context.Context, ref string) error {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent("RefStore.Unlink", trace.WithAttributes(attribute.String("ref", ref)))
	}

	return w.Store.Unlink(ctx, ref)
}
