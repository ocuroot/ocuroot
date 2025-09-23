package work

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/gittools"
	"github.com/ocuroot/ocuroot/client"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/oklog/ulid/v2"
)

func (w *Worker) CopyInWorktree(ctx context.Context, todo Work) (*Worker, func(), error) {
	r, err := gittools.Open(w.Tracker.RepoPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open repo: %w", err)
	}

	if err := os.MkdirAll(workTreeBaseDir(), 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to mkdir: %w", err)
	}

	workTreePath := path.Join(workTreeBaseDir(), ulid.MustNew(ulid.Now(), rand.Reader).String())

	// TODO: Support other repos, using the stored remotes
	if todo.Ref.Repo != w.Tracker.Ref.Repo {
		return nil, nil, fmt.Errorf("todo ref repo %s does not match worker ref repo %s", todo.Ref.Repo, w.Tracker.Ref.Repo)
	}

	// TODO: Estimate the size of the worktree and don't create the worktree if it'll fill the remaining space on disk
	// Size of objects in a tree: git ls-tree -r --format "%(objectsize)"
	// Free space: https://stackoverflow.com/questions/20108520/get-amount-of-free-disk-space-using-go
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

	fmt.Println(workTreePath)

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
	} else {
		fmt.Println("Confirmed that the commit was as expected")
	}

	return newWorker, func() {
		os.RemoveAll(workTreePath)
	}, nil
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
		if t.WorkType != WorkTypeRun && t.WorkType != WorkTypeOp {
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
		fmt.Println("Processing group:", g)

		// TODO: We may want an option to do this in-place for the sake of performance
		// So just a checkout of the appropriate commit, then `git reset --hard`, `git clean -fdxx`
		newWorker, cleanup, err := w.CopyInWorktree(ctx, workGroups[g][0])
		if err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}
		defer cleanup()

		for _, t := range workGroups[g] {
			if t.WorkType == WorkTypeRun {
				log.Info("Processing run", "run", t.Ref.String())
				fmt.Println("Processing run:", t.Ref.String())
				fmt.Println(newWorker.Tracker.Commit)
				if err := newWorker.ExecuteWork(ctx, []Work{t}); err != nil {
					return fmt.Errorf("failed to execute work: %w", err)
				}
			}
		}
	}
	return nil
}

func workTreeBaseDir() string {
	// TODO: Allow this whole thing to be overriden
	return path.Join(client.HomeDir(), "worktrees")
}
