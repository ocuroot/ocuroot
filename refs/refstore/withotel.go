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

// AddDependency implements Store.
func (w *WithOtel) AddDependency(ctx context.Context, ref string, dependency string) error {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent("RefStore.AddDependency", trace.WithAttributes(attribute.String("ref", ref), attribute.String("dependency", dependency)))
	}

	return w.Store.AddDependency(ctx, ref, dependency)
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

// RemoveDependency implements Store.
func (w *WithOtel) RemoveDependency(ctx context.Context, ref string, dependency string) error {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent("RefStore.RemoveDependency", trace.WithAttributes(attribute.String("ref", ref), attribute.String("dependency", dependency)))
	}

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

func (w *WithOtel) AddSupportFiles(ctx context.Context, files map[string]string) error {
	if gitSupportFileWriter, ok := w.Store.(GitSupportFileWriter); ok {
		_, span := tracer.Start(
			ctx,
			"RefStore.AddSupportFiles",
		)
		defer span.End()

		return gitSupportFileWriter.AddSupportFiles(ctx, files)
	}
	return nil
}

var _ GitRepo = (*GitRepoWrapperWithOtel)(nil)

func GitRepoWithOtel(repo GitRepo) GitRepo {
	if DEBUG_TRACES {
		return &GitRepoWrapperWithOtel{
			r: repo,
		}
	}
	return repo
}

type GitRepoWrapperWithOtel struct {
	r GitRepo
}

// Branch implements GitRepo.
func (g *GitRepoWrapperWithOtel) Branch() string {
	return g.r.Branch()
}

// RepoPath implements GitRepo.
func (g *GitRepoWrapperWithOtel) RepoPath() string {
	return g.r.RepoPath()
}

// add implements GitRepo.
func (g *GitRepoWrapperWithOtel) add(ctx context.Context, paths []string) error {
	_, span := tracer.Start(ctx, "git.add", trace.WithAttributes(
		attribute.StringSlice("paths", paths),
	))
	defer span.End()

	return g.r.add(ctx, paths)
}

// checkStagedFiles implements GitRepo.
func (g *GitRepoWrapperWithOtel) checkStagedFiles() error {
	return g.r.checkStagedFiles()
}

// commit implements GitRepo.
func (g *GitRepoWrapperWithOtel) commit(ctx context.Context, message string) error {
	_, span := tracer.Start(ctx, "git.commit", trace.WithAttributes(
		attribute.String("message", message),
	))
	defer span.End()

	return g.r.commit(ctx, message)
}

// pull implements GitRepo.
func (g *GitRepoWrapperWithOtel) pull(ctx context.Context) error {
	_, span := tracer.Start(ctx, "git.pull", trace.WithAttributes(
		attribute.String("remote", "origin"),
		attribute.String("branch", g.r.Branch()),
	))
	defer span.End()

	return g.r.pull(ctx)
}

// push implements GitRepo.
func (g *GitRepoWrapperWithOtel) push(ctx context.Context, remote string) error {
	_, span := tracer.Start(ctx, "git.push", trace.WithAttributes(
		attribute.String("remote", remote),
	))
	defer span.End()

	return g.r.push(ctx, remote)
}
