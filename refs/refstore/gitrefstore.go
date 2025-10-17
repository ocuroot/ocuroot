package refstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/ocuroot/ocuroot/refs"
	"go.opentelemetry.io/otel/attribute"

	"go.opentelemetry.io/otel/trace"
)

type GitRefStoreConfig struct {
	GitRepoConfig

	PathPrefix string
}

func NewGitRefStore(
	baseDir string,
	tags map[string]struct{},
	remote string,
	branch string,
	cfg GitRefStoreConfig,
) (*GitRefStore, error) {
	// Create a branch-specific path to avoid FSRefStore collision
	branchSpecificPath := filepath.Join(baseDir, "branches", branch)

	r, err := NewGitRepoForRemote(branchSpecificPath, remote, branch, cfg.GitRepoConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create git ref store: %w", err)
	}

	fsStore, err := NewFSRefStore(
		filepath.Join(r.RepoPath(), cfg.PathPrefix),
		tags,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create state store: %w", err)
	}

	storeInfoFile := filepath.Join(cfg.PathPrefix, storeInfoFile)

	addInfoFile, err := needsCommit(r, storeInfoFile)
	if err != nil {
		return nil, fmt.Errorf("failed to check if info file needs commit: %w", err)
	}

	r = GitRepoWithOtel(r)
	g := &GitRefStore{
		s:          fsStore,
		g:          r,
		pathPrefix: cfg.PathPrefix,
		lastPull:   time.Now(),
	}

	if addInfoFile {
		err = g.applyFilesAsNeeded(context.Background(), []string{storeInfoFile}, "add store info file")
		if err != nil {
			return nil, fmt.Errorf("failed to add store info file: %w", err)
		}
	}

	return g, nil
}

// needsCommit checks if a file needs to be committed to git.
// It returns true if:
// - The file exists in the working directory AND is not yet tracked in git, OR
// - The file exists AND has uncommitted changes
func needsCommit(r GitRepo, relativeFilePath string) (bool, error) {
	wrapper, ok := r.(*GitRepoWrapper)
	if !ok {
		return false, fmt.Errorf("expected GitRepoWrapper, got %T", r)
	}

	infoFilePath := filepath.Join(r.RepoPath(), relativeFilePath)

	// Check if file exists in working directory
	if _, err := os.Stat(infoFilePath); err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, no need to add
			return false, nil
		}
		return false, fmt.Errorf("failed to check file: %w", err)
	}

	// File exists, check if it's tracked in git
	stdout, stderr, err := wrapper.g.Client.Exec("ls-files", relativeFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to check if file is tracked: %w\n%s", err, string(stderr))
	}

	isTracked := len(strings.TrimSpace(string(stdout))) > 0

	if !isTracked {
		// File exists but not tracked - needs to be added
		return true, nil
	}

	// File is tracked, check if it has uncommitted changes
	stdout, stderr, err = wrapper.g.Client.Exec("diff", "--name-only", "HEAD", "--", relativeFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to check file changes: %w\n%s", err, string(stderr))
	}

	hasChanges := len(strings.TrimSpace(string(stdout))) > 0
	return hasChanges, nil
}

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
	s          *FSStateStore
	g          GitRepo
	pathPrefix string

	lastPull           time.Time
	transactionMessage string
	transactionStarted bool
	transactionSteps   []string
	transactionFiles   []string
}

var _ GitSupportFileWriter = (*GitRefStore)(nil)
var _ Store = (*GitRefStore)(nil)

func getStatePath(baseDir, remote string) (string, error) {
	p := GitURLToValidPath(remote)
	return filepath.Join(baseDir, "state", p), nil
}

func (g *GitRefStore) Info() StoreInfo {
	return g.s.Info()
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
		paths = append(paths, filepath.Join(g.pathPrefix, path))
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

	refFile := g.s.pathToRef(refParsed)
	refFile = filepath.Join(g.pathPrefix, refFile)

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
