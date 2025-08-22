package refstore

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/ocuroot/ocuroot/refs"
	"go.opentelemetry.io/otel/attribute"

	"go.opentelemetry.io/otel/trace"
)

// CheckedStagedFiles uses `git add .` to validate that we have the correct set of staged files
// for each commit.
// Enabling this will make commits slower, but may be useful for debugging status problems.
var CheckStagedFiles = os.Getenv("OCUROOT_CHECK_STAGED_FILES") != ""

type GitSupportFileWriter interface {
	// AddSupportFiles creates a set of files in the git repository outside of
	// the state store contents.
	// It can be used to add CI configuration files to a repo, for example
	AddSupportFiles(ctx context.Context, files map[string]string) error
}

type GitRefStore struct {
	s *FSStateStore
	g GitRepo

	lastPull           time.Time
	transactionMessage string
	transactionStarted bool
	transactionSteps   []string
	transactionFiles   []string
}

var _ GitSupportFileWriter = (*GitRefStore)(nil)
var _ Store = (*GitRefStore)(nil)

func NewGitRefStore(
	baseDir string,
	remote string,
	branch string,
) (*GitRefStore, error) {
	// Create a branch-specific path to avoid FSRefStore collision
	branchSpecificPath := filepath.Join(baseDir, "branches", branch)

	r, err := NewGitRepoForRemote(branchSpecificPath, remote, branch)
	if err != nil {
		return nil, fmt.Errorf("failed to create git ref store: %w", err)
	}
	r = GitRepoWithOtel(r)

	var addInfoFile bool
	if _, err = os.Stat(filepath.Join(r.RepoPath(), storeInfoFile)); err != nil {
		if os.IsNotExist(err) {
			addInfoFile = true
		} else {
			return nil, fmt.Errorf("failed to check for store info file: %w", err)
		}
	}

	fsStore, err := NewFSRefStore(
		r.RepoPath(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create state store: %w", err)
	}

	g := &GitRefStore{
		s: fsStore,
		g: r,

		lastPull: time.Now(),
	}

	if addInfoFile {
		err = g.applyFilesAsNeeded(context.Background(), []string{storeInfoFile}, "add store info file")
		if err != nil {
			return nil, fmt.Errorf("failed to add store info file: %w", err)
		}
	}

	return g, nil
}

func getStatePath(baseDir, remote string) (string, error) {
	remoteURL, err := url.Parse(remote)
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, "state", remoteURL.Host, remoteURL.Path), nil
}

func (g *GitRefStore) StartTransaction(ctx context.Context, message string) error {
	g.transactionStarted = true
	g.transactionMessage = message
	return nil
}

func (g *GitRefStore) CommitTransaction(ctx context.Context) error {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.SetAttributes(
			attribute.StringSlice("transaction.steps", g.transactionSteps),
			attribute.StringSlice("transaction.files", g.transactionFiles),
		)
	}

	if err := g.apply(ctx, g.transactionFiles, strings.Join(append([]string{g.transactionMessage}, g.transactionSteps...), "\n")); err != nil {
		return fmt.Errorf("failed to apply transaction: %w", err)
	}

	g.transactionMessage = ""
	g.transactionStarted = false
	g.transactionSteps = nil
	g.transactionFiles = nil
	return nil
}

func (g *GitRefStore) applyFilesAsNeeded(ctx context.Context, paths []string, message string) error {
	if g.transactionStarted {
		g.transactionSteps = append(g.transactionSteps, message)
		g.transactionFiles = append(g.transactionFiles, paths...)
		return nil
	}
	return g.apply(ctx, paths, message)
}

func (g *GitRefStore) applyAsNeeded(ctx context.Context, refs []string, message string) error {
	var paths []string
	for _, ref := range refs {
		path, err := g.s.ActualPath(ref)
		if err != nil {
			return err
		}
		paths = append(paths, path)
	}

	return g.applyFilesAsNeeded(ctx, paths, message)
}

func (g *GitRefStore) apply(ctx context.Context, paths []string, message string) error {
	if err := g.pullWithoutDebounce(ctx); err != nil {
		return err
	}

	if err := g.g.add(ctx, paths); err != nil {
		return err
	}

	if err := g.g.checkStagedFiles(); err != nil {
		return err
	}

	stack := debug.Stack()
	if err := g.g.commit(ctx, message+"\n\n"+string(stack)); err != nil {
		// If nothing has changed, ignore the error
		if strings.Contains(err.Error(), "nothing to commit") {
			return nil
		}
		return err
	}
	return g.g.push(ctx, "origin")
}

func (g *GitRefStore) Close() error {
	return g.s.Close()
}

func (g *GitRefStore) Get(ctx context.Context, ref string, v any) error {
	// Make sure we're up to date
	err := g.pull(ctx)
	if err != nil {
		return err
	}

	return g.s.Get(ctx, ref, v)
}

func (g *GitRefStore) Set(ctx context.Context, ref string, v any) error {
	// Make sure we're up to date
	err := g.pull(ctx)
	if err != nil {
		return err
	}

	err = g.s.Set(ctx, ref, v)
	if err != nil {
		return err
	}

	return g.applyAsNeeded(ctx, []string{ref}, "update state at "+ref)
}

func (g *GitRefStore) Delete(ctx context.Context, ref string) error {
	// Make sure we're up to date
	err := g.pull(ctx)
	if err != nil {
		return err
	}

	if err := g.s.Delete(ctx, ref); err != nil {
		return err
	}

	return g.applyAsNeeded(ctx, []string{ref}, "delete state at "+ref)
}

func (g *GitRefStore) Link(ctx context.Context, ref string, target string) error {
	// Make sure we're up to date
	err := g.pull(ctx)
	if err != nil {
		return err
	}
	refParsed, err := refs.Parse(ref)
	if err != nil {
		return err
	}
	refResolved, err := g.ResolveLink(ctx, ref)
	if err != nil {
		return err
	}
	refParsedResolved, err := refs.Parse(refResolved)
	if err != nil {
		return err
	}
	refResolvedFile := g.s.pathToRef(refParsedResolved)
	refFile := g.s.pathToRef(refParsed)

	err = g.s.Link(ctx, ref, target)
	if err != nil {
		return err
	}
	targetResolved, err := g.ResolveLink(ctx, target)
	if err != nil {
		return err
	}

	targetParsed, err := refs.Parse(targetResolved)
	if err != nil {
		return err
	}
	targetFile := g.s.pathToRef(targetParsed)

	return g.applyFilesAsNeeded(ctx, []string{refFile, refResolvedFile, targetFile}, "link "+ref+" to "+target)
}

func (g *GitRefStore) Unlink(ctx context.Context, ref string) error {
	// Make sure we're up to date
	err := g.pull(ctx)
	if err != nil {
		return err
	}

	if err := g.s.Unlink(ctx, ref); err != nil {
		return err
	}

	return g.applyAsNeeded(ctx, []string{ref}, "unlink "+ref)
}

func (g *GitRefStore) GetLinks(ctx context.Context, ref string) ([]string, error) {
	// Make sure we're up to date
	err := g.pull(ctx)
	if err != nil {
		return nil, err
	}

	return g.s.GetLinks(ctx, ref)
}

func (g *GitRefStore) ResolveLink(ctx context.Context, ref string) (string, error) {
	// Make sure we're up to date
	err := g.pull(ctx)
	if err != nil {
		return "", err
	}

	return g.s.ResolveLink(ctx, ref)
}

func (g *GitRefStore) Match(ctx context.Context, glob ...string) ([]string, error) {
	// Make sure we're up to date
	err := g.pull(ctx)
	if err != nil {
		return nil, err
	}

	return g.s.Match(ctx, glob...)
}

func (g *GitRefStore) MatchOptions(ctx context.Context, options MatchOptions, glob ...string) ([]string, error) {
	// Make sure we're up to date
	err := g.pull(ctx)
	if err != nil {
		return nil, err
	}

	return g.s.MatchOptions(ctx, options, glob...)
}

func (g *GitRefStore) AddDependency(ctx context.Context, ref string, dependency string) error {
	err := g.pull(ctx)
	if err != nil {
		return err
	}

	if err := g.s.AddDependency(ctx, ref, dependency); err != nil {
		return err
	}

	dependencyMarkerPath, dependantMarkerPath := g.s.ActualDependencyPaths(ctx, ref, dependency)

	return g.applyFilesAsNeeded(ctx, []string{dependencyMarkerPath, dependantMarkerPath}, "add dependency "+dependency+" to "+ref)
}
func (g *GitRefStore) RemoveDependency(ctx context.Context, ref string, dependency string) error {
	err := g.pull(ctx)
	if err != nil {
		return err
	}

	if err := g.s.RemoveDependency(ctx, ref, dependency); err != nil {
		return err
	}

	dependencyMarkerPath, dependantMarkerPath := g.s.ActualDependencyPaths(ctx, ref, dependency)

	return g.applyFilesAsNeeded(ctx, []string{dependencyMarkerPath, dependantMarkerPath}, "remove dependency "+dependency+" from "+ref)
}
func (g *GitRefStore) GetDependencies(ctx context.Context, ref string) ([]string, error) {
	// Make sure we're up to date
	err := g.pull(ctx)
	if err != nil {
		return nil, err
	}

	return g.s.GetDependencies(ctx, ref)
}

func (g *GitRefStore) GetDependants(ctx context.Context, ref string) ([]string, error) {
	// Make sure we're up to date
	err := g.pull(ctx)
	if err != nil {
		return nil, err
	}

	return g.s.GetDependants(ctx, ref)
}

func (g *GitRefStore) AddSupportFiles(ctx context.Context, files map[string]string) error {
	var paths []string
	for k, v := range files {
		fp := filepath.Join(g.g.RepoPath(), k)
		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			return err
		}

		if err := os.WriteFile(fp, []byte(v), 0644); err != nil {
			return err
		}
		paths = append(paths, fp)
	}

	return g.applyFilesAsNeeded(ctx, paths, "add support files")
}

func (g *GitRefStore) pull(ctx context.Context) error {
	if time.Since(g.lastPull) < 5*time.Second {
		return nil
	}
	return g.pullWithoutDebounce(ctx)
}

func (g *GitRefStore) pullWithoutDebounce(ctx context.Context) error {
	g.lastPull = time.Now()
	return g.g.pull(ctx)
}
