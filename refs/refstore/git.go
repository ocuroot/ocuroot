package refstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/ocuroot/gittools"
)

type GitRepoConfig struct {
	CreateBranch bool

	GitUserName  string
	GitUserEmail string
}

type GitRepo interface {
	RepoPath() string
	Branch() string
	add(ctx context.Context, paths []string) error
	commit(ctx context.Context, message string) error
	pull(ctx context.Context) error
	push(ctx context.Context, remote string) error
	checkStagedFiles() error
}

var _ GitRepo = (*GitRepoWrapper)(nil)

type GitRepoWrapper struct {
	g      *gittools.Repo
	branch string
}

func NewGitRepoForRemote(baseDir, remote, branch string, cfg GitRepoConfig) (GitRepo, error) {
	g, branch, err := getRepoForRemote(baseDir, remote, branch, cfg)
	if err != nil {
		return nil, err
	}
	return &GitRepoWrapper{
		g:      g,
		branch: branch,
	}, nil
}

func getRepoForRemote(baseDir, remote, branch string, cfg GitRepoConfig) (*gittools.Repo, string, error) {
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

	cb, err := r.CurrentBranch()
	if err != nil {
		return nil, "", err
	}

	// Checkout the appropriate branch
	if branch != "" && cb != branch {
		if _, stderr, err := r.Client.Exec("fetch", "--all"); err != nil {
			return nil, "", fmt.Errorf("failed to fetch: %w\n%s", err, string(stderr))
		}

		// Get a list of branches
		var branchExists bool
		branchOutput, branchError, err := r.Client.Exec("branch", "-a", "--list")
		if err != nil {
			return nil, "", fmt.Errorf("failed to list branches: %w\n%s", err, string(branchError))
		}
		branches := strings.Split(strings.TrimSpace(string(branchOutput)), "\n")

		for _, b := range branches {
			b = strings.TrimPrefix(b, "*")
			b = strings.TrimSpace(b)
			b = strings.TrimPrefix(b, "remotes/origin/")

			if b == branch {
				branchExists = true
				break
			}
		}

		if cfg.CreateBranch && !branchExists {
			if err := r.CreateBranch(branch); err != nil {
				return nil, "", err
			}
			if err := r.Push("origin", branch); err != nil {
				return nil, "", err
			}
		}
		if err := r.Checkout(branch); err != nil {
			return nil, "", err
		}
	} else {
		branch = cb
	}

	// Apply user to the final client rather than any intermediates
	r.Client.SetUser(cfg.GitUserName, cfg.GitUserEmail)

	return r, branch, nil
}

func (g *GitRepoWrapper) Branch() string {
	return g.branch
}

func (g *GitRepoWrapper) RepoPath() string {
	return g.g.RepoPath
}

func (g *GitRepoWrapper) pull(ctx context.Context) error {
	return g.g.Pull("origin", g.branch)
}

func (g *GitRepoWrapper) add(ctx context.Context, paths []string) error {
	var (
		knownPaths           []string
		possiblyDeletedPaths []string
	)

	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				possiblyDeletedPaths = append(possiblyDeletedPaths, path)
			} else {
				return fmt.Errorf("failed to check for file: %w", err)
			}
		} else {
			knownPaths = append(knownPaths, path)
		}
	}

	if _, stderr, err := g.g.Client.Exec(append([]string{"add"}, knownPaths...)...); err != nil {
		if strings.Contains(string(stderr), "did not match any files") {
			fmt.Println("didn't match files:\n", string(stderr))
		} else {
			return fmt.Errorf("failed to add files\n%s\n%w", string(stderr), err)
		}
	}

	for _, path := range possiblyDeletedPaths {
		if _, stderr, err := g.g.Client.Exec("add", path); err != nil {
			if strings.Contains(string(stderr), "did not match any files") {
				continue
			}
			return fmt.Errorf("failed to add files\n%s\n%w", string(stderr), err)
		}
	}
	return nil
}

func (g *GitRepoWrapper) commit(ctx context.Context, message string) error {
	return g.g.Commit(message, []string{})
}

func (g *GitRepoWrapper) push(ctx context.Context, remote string) error {
	return g.g.Push(remote, g.branch)
}

func (g *GitRepoWrapper) checkStagedFiles() error {
	if !CheckStagedFiles {
		return nil
	}

	stagedFilesBefore, err := g.getStagedFiles()
	if err != nil {
		return err
	}

	// Add all files to make sure we got them all
	_, stderr, err := g.g.Client.Exec("add", "--all")
	if err != nil {
		if strings.Contains(string(stderr), "did not match any files") {
			return nil
		}
		return fmt.Errorf("failed to add files: %w", err)
	}

	stagedFilesAfter, err := g.getStagedFiles()
	if err != nil {
		return err
	}

	sort.Strings(stagedFilesBefore)
	sort.Strings(stagedFilesAfter)

	if strings.Join(stagedFilesBefore, "\n") != strings.Join(stagedFilesAfter, "\n") {
		fmt.Println("staged files didn't match!")
		debug.PrintStack()
		return fmt.Errorf("staged files did not match\n%s", cmp.Diff(stagedFilesBefore, stagedFilesAfter))
	}

	return nil
}

func (g *GitRepoWrapper) getStagedFiles() ([]string, error) {
	stdout, stderr, err := g.g.Client.Exec("diff", "--cached", "--name-only")
	if err != nil {
		return nil, fmt.Errorf("failed to get staged files: %w\n%s", err, string(stderr))
	}

	var stagedFiles []string
	if len(stdout) > 0 {
		stagedFiles = strings.Split(strings.TrimSpace(string(stdout)), "\n")
	}

	return stagedFiles, nil
}
