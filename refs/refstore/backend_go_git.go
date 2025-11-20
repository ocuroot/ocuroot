package refstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-billy/v6/util"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/storage/memory"
)

func NewGoGitBackend(
	ctx context.Context,
	baseDir string,
	remoteURL string,
	branch string,
	gitUserName string,
	gitUserEmail string,
) (DocumentBackend, error) {

	wt := memfs.New()

	r, err := cloneOrInitializeBranch(remoteURL, branch, wt, gitUserName, gitUserEmail)
	if err != nil {
		return nil, err
	}

	return &goGitBackend{
		repo:         r,
		branch:       branch,
		wt:           wt,
		remoteURL:    remoteURL,
		gitUserName:  gitUserName,
		gitUserEmail: gitUserEmail,
	}, nil
}

// cloneOrInitializeBranch attempts to clone a branch, or initializes it if it doesn't exist.
// Handles race conditions where multiple workers may try to initialize simultaneously.
func cloneOrInitializeBranch(
	remoteURL string,
	branch string,
	wt billy.Filesystem,
	gitUserName string,
	gitUserEmail string,
) (*git.Repository, error) {
	// Try to clone the branch
	r, err := git.Clone(memory.NewStorage(), wt, &git.CloneOptions{
		URL:           remoteURL,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
	})
	if err == nil {
		// Branch exists, clone succeeded
		return r, nil
	}

	// Check if the error is because the branch doesn't exist
	if !errors.Is(err, plumbing.ErrReferenceNotFound) && !errors.Is(err, transport.ErrEmptyRemoteRepository) {
		return nil, fmt.Errorf("cloning repo: %w", err)
	}

	// Branch doesn't exist, initialize it
	return initializeNewBranch(remoteURL, branch, wt, gitUserName, gitUserEmail)
}

// initializeNewBranch creates a new branch with an initial commit and pushes it to the remote.
// Handles race conditions where another worker may have initialized the branch concurrently.
func initializeNewBranch(
	remoteURL string,
	branch string,
	wt billy.Filesystem,
	gitUserName string,
	gitUserEmail string,
) (*git.Repository, error) {
	// Initialize local repo
	r, err := git.Init(memory.NewStorage(), git.WithWorkTree(wt))
	if err != nil {
		return nil, fmt.Errorf("initializing repo: %w", err)
	}

	// Add the remote
	_, err = r.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{remoteURL},
	})
	if err != nil {
		return nil, fmt.Errorf("creating remote: %w", err)
	}

	// Get worktree
	w, err := r.Worktree()
	if err != nil {
		return nil, fmt.Errorf("getting worktree: %w", err)
	}

	// Create initial commit on default branch first
	hash, err := w.Commit("Initial commit", &git.CommitOptions{
		AllowEmptyCommits: true,
		Author: &object.Signature{
			Name:  gitUserName,
			Email: gitUserEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("committing initial commit: %w", err)
	}

	// Create and checkout the target branch from the initial commit
	branchRef := plumbing.NewBranchReferenceName(branch)
	ref := plumbing.NewHashReference(branchRef, hash)
	err = r.Storer.SetReference(ref)
	if err != nil {
		return nil, fmt.Errorf("setting branch reference: %w", err)
	}

	err = w.Checkout(&git.CheckoutOptions{
		Branch: branchRef,
	})
	if err != nil {
		return nil, fmt.Errorf("checking out branch: %w", err)
	}

	// Push to establish the branch on remote
	err = r.Push(&git.PushOptions{})
	if err != nil {
		// If push fails, another worker may have initialized the repo concurrently
		if err := handleInitializationRace(r, branch); err != nil {
			return nil, err
		}
	}

	return r, nil
}

// handleInitializationRace handles the case where multiple workers try to initialize
// the same branch simultaneously. If another worker succeeded, sync with their commit.
func handleInitializationRace(r *git.Repository, branch string) error {
	// Fetch to get the latest remote state
	fetchErr := r.Fetch(&git.FetchOptions{})
	if fetchErr != nil && !errors.Is(fetchErr, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("fetching after push failure: %w", fetchErr)
	}

	// Check if the branch now exists on remote (another worker created it)
	remoteRefName := plumbing.NewRemoteReferenceName("origin", branch)
	_, refErr := r.Reference(remoteRefName, true)
	if refErr != nil {
		// Branch still doesn't exist, the push genuinely failed
		return fmt.Errorf("pushing initial commit: branch not found after fetch")
	}

	// Branch exists now - another worker won the race
	// Pull the remote branch to sync our local state
	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree after race: %w", err)
	}

	err = w.Pull(&git.PullOptions{
		ReferenceName: plumbing.NewBranchReferenceName(branch),
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("pulling after race: %w", err)
	}

	log.Info("another worker initialized the repository first, synced successfully", "branch", branch)
	return nil
}

var _ DocumentBackend = (*goGitBackend)(nil)

type goGitBackend struct {
	repo   *git.Repository
	branch string
	wt     billy.Filesystem

	remoteURL    string
	gitUserName  string
	gitUserEmail string

	mu sync.Mutex // Protects all git operations
}

// recreateRepo recreates the repository from scratch by cloning again.
// Must be called with g.mu held.
func (g *goGitBackend) recreateRepo() error {
	log.Info("recreating repository from scratch", "branch", g.branch)

	// Create new in-memory filesystem
	newWt := memfs.New()

	// Clone the repository again
	r, err := cloneOrInitializeBranch(g.remoteURL, g.branch, newWt, g.gitUserName, g.gitUserEmail)
	if err != nil {
		return fmt.Errorf("recreating repo: %w", err)
	}

	// Replace the old repo and filesystem
	g.repo = r
	g.wt = newWt

	return nil
}

// refresh pulls the latest changes from the remote branch.
// Must be called with g.mu held.
func (g *goGitBackend) refresh() error {
	wt, err := g.repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}

	if err := wt.Pull(&git.PullOptions{
		ReferenceName: plumbing.NewBranchReferenceName(g.branch),
	}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {

		// Check for pipe errors - recreate the repo from scratch
		if strings.Contains(err.Error(), "closed pipe") || strings.Contains(err.Error(), "broken pipe") {
			log.Warn("detected pipe error during pull, recreating repository", "error", err)
			if recreateErr := g.recreateRepo(); recreateErr != nil {
				return fmt.Errorf("failed to recreate repo after pipe error: %w", recreateErr)
			}
			return nil
		}

		if errors.Is(err, git.ErrNonFastForwardUpdate) {
			log.Info("non-fast-forward update", "stack", string(debug.Stack()))
			// Get local HEAD
			localRef, err := g.repo.Head()
			if err != nil {
				log.Error("failed to get local HEAD", "error", err)
			} else {
				localCommit, err := g.repo.CommitObject(localRef.Hash())
				if err != nil {
					log.Error("failed to get local commit", "error", err)
				} else {
					log.Info("non-fast-forward update detected",
						"branch", g.branch,
						"local_commit", localRef.Hash().String()[:8],
						"local_message", strings.Split(localCommit.Message, "\n")[0],
						"local_author", localCommit.Author.Name,
						"local_time", localCommit.Author.When.Format(time.RFC3339))
				}
			}

			// Fetch to get remote state
			if err := g.repo.Fetch(&git.FetchOptions{}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
				log.Error("failed to fetch remote", "error", err)
			} else {
				// Find remote tracking branch
				remoteRefName := plumbing.NewRemoteReferenceName("origin", g.branch)
				remoteRef, err := g.repo.Reference(remoteRefName, true)
				if err != nil {
					log.Error("failed to get remote reference", "error", err)
				} else {
					remoteCommit, err := g.repo.CommitObject(remoteRef.Hash())
					if err != nil {
						log.Error("failed to get remote commit", "error", err)
					} else {
						log.Info("remote branch state",
							"remote_commit", remoteRef.Hash().String()[:8],
							"remote_message", strings.Split(remoteCommit.Message, "\n")[0],
							"remote_author", remoteCommit.Author.Name,
							"remote_time", remoteCommit.Author.When.Format(time.RFC3339))

						// Calculate divergence
						if localRef != nil {
							localCommit, _ := g.repo.CommitObject(localRef.Hash())
							if localCommit != nil {
								ahead, behind := calculateDivergence(g.repo, localCommit, remoteCommit)
								log.Info("branch divergence",
									"commits_ahead", ahead,
									"commits_behind", behind,
									"action_needed", "local changes need to be rebased or merged")
							}
						}
					}
				}
			}
		}

		return fmt.Errorf("pulling: %w", err)
	}

	return nil
}

// Get implements DocumentBackend.
func (g *goGitBackend) Get(ctx context.Context, paths []string) ([]GetResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if err := g.refresh(); err != nil {
		return nil, err
	}

	wt, err := g.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("get worktree: %w", err)
	}

	var out []GetResult
	for _, p := range paths {
		f, err := wt.Filesystem.Open(p)
		if err != nil {
			out = append(out, GetResult{
				Path: p,
			})
			continue
		}

		res := GetResult{
			Path: p,
		}

		body, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(body, &res.Doc)
		if err != nil {
			return nil, err
		}

		out = append(out, res)
	}

	return out, nil
}

// GetBytes implements DocumentBackend.
func (g *goGitBackend) GetBytes(ctx context.Context, path string) ([]byte, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if err := g.refresh(); err != nil {
		return nil, err
	}

	f, err := g.wt.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open file: %w", err)
	}

	body, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read content: %w", err)
	}

	return body, nil
}

// Match implements DocumentBackend.
func (g *goGitBackend) Match(ctx context.Context, reqs []MatchRequest) ([]string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if err := g.refresh(); err != nil {
		return nil, err
	}

	compiledReqs, err := compileMatchRequests(reqs)
	if err != nil {
		return nil, fmt.Errorf("compile match requests: %w", err)
	}

	var matchingRefs []string
	err = util.Walk(g.wt, ".", func(p string, d os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel("", p)
		if err != nil {
			return err
		}

		candidate := relPath
		for _, g := range compiledReqs {
			c := candidate
			if g.prefix != "" {
				if !strings.HasPrefix(c, g.prefix) {
					continue
				}
				c = strings.TrimPrefix(c, g.prefix)
			}
			suffixes := g.suffixes
			if len(suffixes) == 0 {
				suffixes = []string{""}
			}
			for _, suffix := range suffixes {
				hasSuffix := strings.HasSuffix(c, suffix)
				globMatch := g.compiledGlob.Match(strings.TrimSuffix(c, suffix))
				if hasSuffix && globMatch {
					matchingRefs = append(matchingRefs, candidate)
					return nil
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking for matches: %w", err)
	}

	return matchingRefs, nil
}

// Set implements DocumentBackend.
func (g *goGitBackend) Set(ctx context.Context, message string, reqs []SetRequest) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Retry refresh on non-fast-forward errors
	var err error
	for i := 0; i < 3; i++ {
		err = g.refresh()
		if err == nil {
			break
		}
		if errors.Is(err, git.ErrNonFastForwardUpdate) {
			time.Sleep(time.Millisecond)
			continue
		}
		return err
	}
	if err != nil {
		return err
	}

	w, err := g.repo.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	for _, req := range reqs {
		if req.Doc != nil {
			f, err := g.wt.Create(req.Path)
			if err != nil {
				return fmt.Errorf("creating file: %w", err)
			}

			body, err := json.Marshal(req.Doc)
			if err != nil {
				return fmt.Errorf("marshaling document: %w", err)
			}

			_, err = f.Write(body)
			if err != nil {
				return fmt.Errorf("writing file: %w", err)
			}

			err = f.Close()
			if err != nil {
				return fmt.Errorf("closing file: %w", err)
			}
			_, err = w.Add(req.Path)
			if err != nil {
				return fmt.Errorf("adding file %q: %w", req.Path, err)
			}
		} else if _, se := g.wt.Stat(req.Path); se == nil {
			err := g.wt.Remove(req.Path)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("deleting file: %w", err)
			}

			_, err = w.Add(req.Path)
			if err != nil {
				return fmt.Errorf("adding file %q: %w", req.Path, err)
			}
		}

	}

	s, err := w.Status()
	if err != nil {
		return fmt.Errorf("check status: %w", err)
	}
	// Nothing to do
	if s.IsClean() {
		return nil
	}

	_, err = w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  g.gitUserName,
			Email: g.gitUserEmail,
		},
	})
	if err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	err = g.repo.Push(&git.PushOptions{})
	if err != nil {
		// Check for pipe errors - recreate and retry
		if strings.Contains(err.Error(), "closed pipe") || strings.Contains(err.Error(), "broken pipe") {
			log.Warn("detected pipe error during push, recreating repository", "error", err)
			if recreateErr := g.recreateRepo(); recreateErr != nil {
				return fmt.Errorf("failed to recreate repo after pipe error: %w", recreateErr)
			}
			// After recreating, we need to refresh and retry the whole operation
			return fmt.Errorf("repository recreated due to pipe error, please retry operation")
		}

		if !strings.Contains(err.Error(), "non-fast-forward update") {
			return fmt.Errorf("pushing: %w", err)
		}

		if err := g.rebaseAndPushWithRetry(w); err != nil {
			return fmt.Errorf("retry push: %w", err)
		}
	}

	return nil
}

// rebaseAndPushWithRetry attempts to rebase and push with exponential backoff.
// It will retry up to 5 times with increasing delays between attempts.
func (g *goGitBackend) rebaseAndPushWithRetry(w *git.Worktree) error {
	var errs []error
	backoff := time.Millisecond
	for i := 0; i < 5; i++ {
		if i > 0 {
			time.Sleep(backoff)
			backoff *= 2
		}

		err := g.rebaseAndPush(w)
		if err == nil {
			return nil
		}

		errs = append(errs, fmt.Errorf("attempt %d: %w", i+1, err))

		// If it's still a non-fast-forward error, retry
		if strings.Contains(err.Error(), "non-fast-forward update") {
			continue
		}

		// For other errors, fail immediately
		return errors.Join(errs...)
	}

	return fmt.Errorf("failed to rebase and push after 5 attempts: %w", errors.Join(errs...))
}

// rebaseAndPush handles non-fast-forward push errors by merging remote changes
// and pushing again.
func (g *goGitBackend) rebaseAndPush(w *git.Worktree) error {
	// Fetch the latest remote state
	err := g.repo.Fetch(&git.FetchOptions{})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("fetching: %w", err)
	}

	// Find the remote tracking branch
	var remoteRef *plumbing.Reference
	refs, err := g.repo.References()
	if err != nil {
		return fmt.Errorf("getting references: %w", err)
	}
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsRemote() {
			remoteRef = ref
			return nil
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("iterating references: %w", err)
	}
	if remoteRef == nil {
		return fmt.Errorf("no remote reference found")
	}

	// Get both commits for merge
	remoteCommit, err := g.repo.CommitObject(remoteRef.Hash())
	if err != nil {
		return fmt.Errorf("getting remote commit: %w", err)
	}

	localRef, err := g.repo.Head()
	if err != nil {
		return fmt.Errorf("getting head: %w", err)
	}
	localCommit, err := g.repo.CommitObject(localRef.Hash())
	if err != nil {
		return fmt.Errorf("getting local commit: %w", err)
	}

	// Check for conflicts by comparing files
	remoteTree, err := remoteCommit.Tree()
	if err != nil {
		return fmt.Errorf("getting remote tree: %w", err)
	}
	localTree, err := localCommit.Tree()
	if err != nil {
		return fmt.Errorf("getting local tree: %w", err)
	}

	// Build maps of file paths to contents
	remoteFiles := make(map[string]string)
	err = remoteTree.Files().ForEach(func(f *object.File) error {
		contents, err := f.Contents()
		if err != nil {
			return fmt.Errorf("reading contents: %w", err)
		}
		remoteFiles[f.Name] = contents
		return nil
	})
	if err != nil {
		return fmt.Errorf("reading remote files: %w", err)
	}

	localFiles := make(map[string]string)
	err = localTree.Files().ForEach(func(f *object.File) error {
		contents, err := f.Contents()
		if err != nil {
			return fmt.Errorf("reading contents: %w", err)
		}
		localFiles[f.Name] = contents
		return nil
	})
	if err != nil {
		return fmt.Errorf("reading local files: %w", err)
	}

	// Check for conflicts (same file, different content)
	for path, localContent := range localFiles {
		if remoteContent, exists := remoteFiles[path]; exists {
			if localContent != remoteContent {
				return fmt.Errorf("merge conflict in file %s", path)
			}
		}
	}

	// Apply all files (remote first, then local to prefer local changes)
	for path, content := range remoteFiles {
		file, err := g.wt.Create(path)
		if err != nil {
			return fmt.Errorf("creating file %s: %w", path, err)
		}
		_, err = file.Write([]byte(content))
		if err != nil {
			return fmt.Errorf("writing file %s: %w", path, err)
		}
		file.Close()
	}
	for path, content := range localFiles {
		file, err := g.wt.Create(path)
		if err != nil {
			return fmt.Errorf("creating file %s: %w", path, err)
		}
		_, err = file.Write([]byte(content))
		if err != nil {
			return fmt.Errorf("writing file %s: %w", path, err)
		}
		file.Close()
		_, err = w.Add(path)
		if err != nil {
			return fmt.Errorf("adding file %q: %w", path, err)
		}
	}

	// Create merge commit with remote as first parent (base)
	// This ensures the merge is based on what the remote has
	_, err = w.Commit("Merge remote changes", &git.CommitOptions{
		Author:            &localCommit.Author,
		Parents:           []plumbing.Hash{remoteCommit.Hash, localCommit.Hash},
		AllowEmptyCommits: true,
	})
	if err != nil {
		return fmt.Errorf("committing merge: %w", err)
	}

	// Try to push again
	err = g.repo.Push(&git.PushOptions{})
	if err != nil {
		return fmt.Errorf("pushing after merge: %w", err)
	}

	return nil
}

// SetBytes implements DocumentBackend.
func (g *goGitBackend) SetBytes(ctx context.Context, path string, content []byte) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if err := g.refresh(); err != nil {
		return err
	}

	w, err := g.repo.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	if content != nil {
		f, err := g.wt.Create(path)
		if err != nil {
			return fmt.Errorf("creating file: %w", err)
		}

		_, err = f.Write(content)
		if err != nil {
			return fmt.Errorf("writing file: %w", err)
		}

		err = f.Close()
		if err != nil {
			return fmt.Errorf("closing file: %w", err)
		}

	} else {
		err := g.wt.Remove(path)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("deleting file: %w", err)
		}
	}

	_, err = w.Add(path)
	if err != nil {
		return fmt.Errorf("adding file %q: %w", path, err)
	}

	s, err := w.Status()
	if err != nil {
		return fmt.Errorf("check status: %w", err)
	}
	// Nothing to do
	if s.IsClean() {
		return nil
	}

	_, err = w.Commit("set bytes", &git.CommitOptions{
		Author: &object.Signature{
			Name:  g.gitUserName,
			Email: g.gitUserEmail,
		},
	})
	if err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	err = g.repo.Push(&git.PushOptions{})
	if err != nil {
		// Check for pipe errors - recreate and retry
		if strings.Contains(err.Error(), "closed pipe") || strings.Contains(err.Error(), "broken pipe") {
			log.Warn("detected pipe error during push, recreating repository", "error", err)
			if recreateErr := g.recreateRepo(); recreateErr != nil {
				return fmt.Errorf("failed to recreate repo after pipe error: %w", recreateErr)
			}
			// After recreating, we need to refresh and retry the whole operation
			return fmt.Errorf("repository recreated due to pipe error, please retry operation")
		}

		if !strings.Contains(err.Error(), "non-fast-forward update") {
			return fmt.Errorf("pushing: %w", err)
		}

		if err := g.rebaseAndPushWithRetry(w); err != nil {
			return fmt.Errorf("retry push: %w", err)
		}
	}

	return nil
}

// calculateDivergence calculates how many commits ahead and behind the local branch is
// compared to the remote branch.
func calculateDivergence(repo *git.Repository, local, remote *object.Commit) (ahead, behind int) {
	// Find common ancestor
	commonAncestor, err := local.MergeBase(remote)
	if err != nil || len(commonAncestor) == 0 {
		// If we can't find a common ancestor, just return 0,0
		return 0, 0
	}

	base := commonAncestor[0]

	// Count commits from base to local (ahead)
	localIter, err := repo.Log(&git.LogOptions{From: local.Hash})
	if err == nil {
		err = localIter.ForEach(func(c *object.Commit) error {
			if c.Hash == base.Hash {
				return io.EOF
			}
			ahead++
			return nil
		})
		if err != nil && err != io.EOF {
			ahead = 0
		}
	}

	// Count commits from base to remote (behind)
	remoteIter, err := repo.Log(&git.LogOptions{From: remote.Hash})
	if err == nil {
		err = remoteIter.ForEach(func(c *object.Commit) error {
			if c.Hash == base.Hash {
				return io.EOF
			}
			behind++
			return nil
		})
		if err != nil && err != io.EOF {
			behind = 0
		}
	}

	return ahead, behind
}
