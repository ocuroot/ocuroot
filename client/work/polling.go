package work

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/git"
	"github.com/ocuroot/ocuroot/refs"
)

func (w *InRepoWorker) Poll(ctx context.Context) error {
	workerBranch := fmt.Sprintf("refs/heads/%v", w.RepoInfo.Branch)
	repoURL := "file://" + w.RepoInfo.Root
	workerInterval := time.Second // Can afford to poll very frequently when running locally
	log.Info("Using local repository", "path", w.RepoInfo.Root)

	log.Info("Starting worker", "branch", workerBranch, "interval", workerInterval)

	// Create ticker for polling
	ticker := time.NewTicker(workerInterval)

	pollTargets := []git.PollTarget{
		{
			Endpoint: repoURL,
			Branch:   workerBranch,
		},
	}

	if w.Tracker.StoreConfig.Intent != nil && w.Tracker.StoreConfig.Intent.Git != nil {
		pollTargets = append(pollTargets, git.PollTarget{
			Endpoint: w.Tracker.StoreConfig.Intent.Git.RemoteURL,
			Branch:   w.Tracker.StoreConfig.Intent.Git.Branch,
		})
	}

	err := git.PollMultiple(
		ctx,
		pollTargets,
		w.handleCommit,
		ticker.C,
	)
	if err != nil && err != context.Canceled {
		return fmt.Errorf("polling error: %w", err)
	}

	return nil
}

func (w *InRepoWorker) handleCommit(remote string, hash string) {
	log.Info("New commit detected", "remote", remote, "hash", hash)

	repoDir, err := CloneRepo(context.Background(), []string{remote}, hash)
	if err != nil {
		log.Error("failed to clone repo", "remote", remote, "hash", hash, "err", err)
		return
	}

	log.Info("Will work in", "repoDir", repoDir)
	if err := os.Chdir(repoDir); err != nil {
		log.Error("failed to change directory", "hash", hash, "err", err)
		return
	}

	worker, err := NewInRepoWorker(context.Background(), refs.Ref{
		Filename: ".",
	})
	if err != nil {
		log.Error("failed to create worker", "hash", hash, "err", err)
		return
	}
	defer worker.Cleanup()

	if err := worker.Push(context.Background()); err != nil {
		log.Error("failed to handle push", "hash", hash, "err", err)
		return
	}
}
