package work

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/gittools"
	"github.com/ocuroot/ocuroot/client"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/store/models"
	"github.com/oklog/ulid/v2"
	"github.com/ricochet2200/go-disk-usage/du"
)

func (w *Worker) WorkerForWork(ctx context.Context, todo Work) (*Worker, func(), error) {
	if todo.Ref.Repo != w.RepoName {
		wOut, cleanup, err := w.CopyInRepoClone(ctx, todo.Ref, todo.Ref.Repo, todo.Commit)
		if err != nil {
			return nil, nil, fmt.Errorf("worker from clone: %w", err)
		}
		return wOut, cleanup, nil
	}
	// No-op if the same repo and the same commit
	if todo.Commit == w.Tracker.Commit {
		return w, func() {}, nil
	}
	return w.CopyInWorktree(ctx, todo)
}

func (w *Worker) CopyInRepoClone(ctx context.Context, ref refs.Ref, repoName, commit string) (*Worker, func(), error) {
	if err := os.MkdirAll(repoCloneBaseDir(), 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to mkdir: %w", err)
	}

	log.Info("Cloning repo", "ref", ref, "repoName", repoName, "commit", commit)

	var repoInfo models.RepoConfig
	repoRef := fmt.Sprintf("%v/-/repo.ocu.star/@", repoName)
	log.Info("Getting repo info", "repoRef", repoRef)
	err := w.Tracker.State.Get(ctx, repoRef, &repoInfo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get repo info for %s: %w", repoRef, err)
	}

	log.Info("Repo info", "remotes", repoInfo.Remotes, "source", string(repoInfo.Source))

	globals, be, err := w.RepoConfigFromState(ctx, repoName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get repo config: %w", err)
	}

	// Load globals from repo into settings
	w.Settings, err = LoadSettings(be, globals, os.Environ())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load settings: %w", err)
	}

	repoCloneDir := path.Join(repoCloneBaseDir(), ulid.MustNew(ulid.Now(), rand.Reader).String())

	var remotes []string
	// Add discovered fetch URLs
	for _, r := range repoInfo.Remotes {
		remotes = append(remotes, r.URL)
	}
	// Override with configured remotes
	if len(be.RepoRemotes) > 0 {
		remotes = be.RepoRemotes
	}

	var repo *gittools.Repo
	var remoteToError = make(map[string]error)
	for _, remote := range remotes {
		log.Info("Attempting clone", "remote", remote, "repoCloneDir", repoCloneDir)
		repo, err = gittools.NewClient().Clone(remote, repoCloneDir)
		if err != nil {
			log.Error("failed to clone repo", "remote", remote, "repoCloneDir", repoCloneDir, "err", err)
			remoteToError[remote] = err
		} else {
			log.Info("Clone successful")
			break
		}
	}
	if repo == nil {
		return nil, nil, fmt.Errorf("all remotes exhausted trying to clone repo\n%v", remoteToError)
	}

	err = repo.Checkout(commit)
	if err != nil {
		log.Error("failed to checkout commit", "commit", commit, "repoCloneDir", repoCloneDir, "err", err)
		return nil, nil, fmt.Errorf("failed to checkout commit: %w\nin repoDir: %v", err, repoCloneDir)
	}

	wc := *w
	newWorker := &wc
	newWorker.Tracker.Commit = commit
	newWorker.Tracker.RepoPath = repoCloneDir
	newWorker.Tracker.Ref = ref

	// Initialize tracker as needed
	if newWorker.Tracker.Intent == nil || newWorker.Tracker.State == nil {
		// We don't save the repo config here since it should already exist
		err := newWorker.InitTrackerFromSourceRepo(ctx, ref, repoCloneDir, repoCloneDir, false)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to init tracker from source repo: %w", err)
		}
	}

	// Check that the worktree is as expected
	wtr, err := gittools.Open(repoCloneDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open worktree: %w", err)
	}

	head, stderr, err := wtr.Client.Exec("rev-parse", "HEAD")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get head: %w\n%v", err, string(stderr))
	}
	if strings.TrimSpace(string(head)) != commit {
		return nil, nil, fmt.Errorf("commit in worktree does not match expected commit: %s != %s", string(head), commit)
	}

	return newWorker, func() {
		if true {
			return
		}
		os.RemoveAll(repoCloneDir)
	}, nil
}

func (w *Worker) CopyInWorktree(ctx context.Context, todo Work) (*Worker, func(), error) {
	if err := os.MkdirAll(workTreeBaseDir(), 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to mkdir: %w", err)
	}

	r, err := gittools.Open(w.Tracker.RepoPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open repo: %w", err)
	}

	workTreePath := path.Join(workTreeBaseDir(), ulid.MustNew(ulid.Now(), rand.Reader).String())

	// Estimate the size of the worktree and don't create the worktree if it'll fill the remaining space on disk
	spaceAvailable, err := w.checkSpaceForWorktree(r, todo.Commit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to check space for worktree: %w", err)
	}
	if !spaceAvailable {
		return nil, nil, fmt.Errorf("not enough space to create worktree")
	}

	// TODO: We may want an option to do this in-place for the sake of performance
	// So just a checkout of the appropriate commit, then `git reset --hard`, `git clean -fdxx`
	if _, stderr, err := r.Client.Exec("worktree", "add", workTreePath, todo.Commit); err != nil {
		return nil, nil, fmt.Errorf("failed to add worktree: %w\nworkTreePath=%q, commit=%q, todo=%+v\n%v", err, workTreePath, todo.Commit, todo, string(stderr))
	}

	newWorker := &Worker{
		Tracker: w.Tracker,
		Tui:     w.Tui,
	}
	newWorker.Tracker.Commit = todo.Commit
	newWorker.Tracker.RepoPath = workTreePath
	newWorker.Tracker.Ref = todo.Ref

	// Check that the worktree is as expected
	wtr, err := gittools.Open(workTreePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open worktree: %w", err)
	}

	head, stderr, err := wtr.Client.Exec("rev-parse", "HEAD")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get head: %w\n%v", err, string(stderr))
	}
	if strings.TrimSpace(string(head)) != todo.Commit {
		return nil, nil, fmt.Errorf("commit in worktree does not match expected commit: %s != %s", string(head), todo.Commit)
	}

	return newWorker, func() {
		os.RemoveAll(workTreePath)
	}, nil
}

func (w *Worker) checkSpaceForWorktree(repo *gittools.Repo, commit string) (bool, error) {
	counts, stderr, err := repo.Client.Exec("ls-tree", "-r", "--format=%(objectsize)", commit)
	if err != nil {
		return false, fmt.Errorf("failed to get counts: %w\n%v", err, string(stderr))
	}
	var totalSize uint64
	for _, count := range strings.Split(string(counts), "\n") {
		if len(count) == 0 {
			continue
		}
		size, err := strconv.ParseUint(strings.TrimSpace(count), 10, 64)
		if err != nil {
			continue
		}
		totalSize += size
	}

	usage := du.NewDiskUsage(workTreeBaseDir())

	log.Info("worktree space check", "path", workTreeBaseDir(), "worktreeSize", totalSize, "free", usage.Free(), "available", usage.Available())

	return usage.Free() > totalSize, nil
}

func (w *Worker) ExecuteWorkInCleanWorktrees(ctx context.Context, todos []Work) error {
	log.Info("Applying intent diffs")
	for _, t := range todos {
		if t.WorkType == WorkTypeUpdate || t.WorkType == WorkTypeCreate || t.WorkType == WorkTypeDelete {
			if err := w.ApplyIntent(ctx, t.Ref); err != nil {
				return fmt.Errorf("failed to apply intent (%s): %w", t.Ref.String(), err)
			}
		}
	}

	log.Info("Starting release work in clean trees")
	workGroups := make(map[string][]Work)
	for _, t := range todos {
		if t.WorkType != WorkTypeRun && t.WorkType != WorkTypeOp && t.WorkType != WorkTypeRelease {
			continue
		}
		if t.Commit == "" {
			return fmt.Errorf("commit is empty for todo %v", t)
		}

		repoCommitRef := t.Ref.SetFilename("repo.ocu.star").
			SetRelease(t.Commit).
			SetSubPathType(refs.SubPathTypeNone).
			SetSubPath("").
			SetFragment("")

		workGroups[repoCommitRef.String()] = append(workGroups[repoCommitRef.String()], t)
	}
	var sortedGroups []string
	for k := range workGroups {
		sortedGroups = append(sortedGroups, k)
	}
	sort.Strings(sortedGroups)

	for _, g := range sortedGroups {
		log.Info("Processing group", "group", g)

		// Create a new Worker as needed for different repos and commits
		newWorker, cleanup, err := w.WorkerForWork(ctx, workGroups[g][0])
		if err != nil {
			return fmt.Errorf("creating worker: %w", err)
		}
		defer cleanup()

		for _, t := range workGroups[g] {
			if t.WorkType == WorkTypeRun || t.WorkType == WorkTypeOp || t.WorkType == WorkTypeRelease {
				if err := newWorker.ExecuteWork(ctx, []Work{t}); err != nil {
					return fmt.Errorf("failed to execute work: %w", err)
				}
			}
		}
	}
	return nil
}

func repoCloneBaseDir() string {
	// TODO: Allow this to be overriden
	return path.Join(client.HomeDir(), "clones")
}

func workTreeBaseDir() string {
	// TODO: Allow this whole thing to be overriden
	return path.Join(client.HomeDir(), "worktrees")
}
