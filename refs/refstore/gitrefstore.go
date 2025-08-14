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

	"github.com/ocuroot/gittools"
)

type GitSupportFileWriter interface {
	// AddSupportFiles creates a set of files in the git repository outside of
	// the state store contents.
	// It can be used to add CI configuration files to a repo, for example
	AddSupportFiles(ctx context.Context, files map[string]string) error
}

type GitRefStore struct {
	s      *FSStateStore
	g      *gittools.Repo
	branch string

	transactionStarted bool
	transactionSteps   []string

	lastPull time.Time
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

	r, branch, err := getRepoForRemote(branchSpecificPath, remote, branch)
	if err != nil {
		return nil, fmt.Errorf("failed to create git ref store: %w", err)
	}

	fsStore, err := NewFSRefStore(
		r.RepoPath,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create state store: %w", err)
	}

	g := &GitRefStore{
		s:        fsStore,
		g:        r,
		branch:   branch,
		lastPull: time.Now(),
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

func getRepoForRemote(baseDir, remote, branch string) (*gittools.Repo, string, error) {
	statePath, err := getStatePath(baseDir, remote)
	if err != nil {
		return nil, "", err
	}

	var r *gittools.Repo

	var shouldClone bool
	if _, err := os.Stat(filepath.Join(statePath, ".git")); err != nil {
		if os.IsNotExist(err) {
			shouldClone = true
		} else {
			return nil, "", err
		}
	}

	client := gittools.NewClient()

	// Make sure we have a copy of the repo available
	if shouldClone {
		r, err = client.Clone(remote, statePath)
		if err != nil {
			return nil, "", err
		}
	} else {
		r, err = gittools.Open(statePath)
		if err != nil {
			return nil, "", err
		}
	}

	if !shouldClone {
		// Make sure we're up to date
		if err := r.Pull("origin", branch); err != nil {
			// Attempt to clone a fresh copy
			if err := os.RemoveAll(statePath); err != nil {
				return nil, "", err
			}

			r, err = client.Clone(remote, statePath)
			if err != nil {
				return nil, "", err
			}
		}
	}

	// Checkout the appropriate branch
	if branch != "" {
		if err := r.Checkout(branch); err != nil {
			return nil, "", err
		}
	} else {
		branch, err = r.CurrentBranch()
		if err != nil {
			return nil, "", err
		}
	}

	return r, branch, nil
}

func (g *GitRefStore) StartTransaction(ctx context.Context) error {
	g.transactionStarted = true
	return nil
}

func (g *GitRefStore) CommitTransaction(ctx context.Context, message string) error {
	if err := g.apply(ctx, strings.Join(append([]string{message}, g.transactionSteps...), "\n")); err != nil {
		return err
	}

	g.transactionStarted = false
	g.transactionSteps = nil
	return nil
}

func (g *GitRefStore) applyAsNeeded(ctx context.Context, message string) error {
	if g.transactionStarted {
		g.transactionSteps = append(g.transactionSteps, message)
		return nil
	}
	return g.apply(ctx, message)
}

func (g *GitRefStore) pull(ctx context.Context) error {
	if time.Since(g.lastPull) < 5*time.Second {
		return nil
	}
	g.lastPull = time.Now()
	return g.g.Pull("origin", g.branch)
}

func (g *GitRefStore) apply(ctx context.Context, message string) error {
	g.lastPull = time.Now()
	if err := g.g.Pull("origin", g.branch); err != nil {
		return err
	}

	stack := debug.Stack()
	if err := g.g.CommitAll(message + "\n\n" + string(stack)); err != nil {
		// If nothing has changed, ignore the error
		if strings.Contains(err.Error(), "nothing to commit") {
			return nil
		}
		return err
	}
	return g.g.Push("origin", g.branch)
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

	if g.transactionStarted {
		g.transactionSteps = append(g.transactionSteps, "set "+ref)
		return nil
	}

	return g.applyAsNeeded(ctx, "update state at "+ref)
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

	return g.applyAsNeeded(ctx, "delete state at "+ref)
}

func (g *GitRefStore) Link(ctx context.Context, ref string, target string) error {
	// Make sure we're up to date
	err := g.pull(ctx)
	if err != nil {
		return err
	}

	err = g.s.Link(ctx, ref, target)
	if err != nil {
		return err
	}

	return g.applyAsNeeded(ctx, "link "+ref+" to "+target)
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

	return g.applyAsNeeded(ctx, "unlink "+ref)
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

	return g.applyAsNeeded(ctx, "add dependency "+dependency+" to "+ref)
}
func (g *GitRefStore) RemoveDependency(ctx context.Context, ref string, dependency string) error {
	err := g.pull(ctx)
	if err != nil {
		return err
	}

	if err := g.s.RemoveDependency(ctx, ref, dependency); err != nil {
		return err
	}

	return g.applyAsNeeded(ctx, "remove dependency "+dependency+" from "+ref)
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
	for k, v := range files {
		fp := filepath.Join(g.g.RepoPath, k)
		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			return err
		}

		if err := os.WriteFile(fp, []byte(v), 0644); err != nil {
			return err
		}
	}

	if g.transactionStarted {
		g.transactionSteps = append(g.transactionSteps, "add support files")
		return nil
	}

	return g.applyAsNeeded(ctx, "add support files")
}
