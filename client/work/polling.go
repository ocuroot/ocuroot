package work

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/git"
)

func (w *InRepoWorker) Poll(ctx context.Context) error {
	workerBranch := fmt.Sprintf("refs/heads/%v", w.RepoInfo.Branch)
	repoURL := "file://" + w.RepoInfo.Root
	workerInterval := time.Second // Can afford to poll very frequently when running locally
	log.Info("Using local repository", "path", w.RepoInfo.Root)

	log.Info("Starting worker", "branch", workerBranch, "interval", workerInterval)

	// Create ticker for polling
	ticker := time.NewTicker(workerInterval)

	err := git.PollMultiple(
		ctx,
		[]git.PollTarget{
			{
				Endpoint: repoURL,
				Branch:   workerBranch,
			},
		},
		w.handleCommit,
		ticker.C,
	)
	if err != nil && err != context.Canceled {
		return fmt.Errorf("polling error: %w", err)
	}

	return nil
}

func (w *InRepoWorker) handleCommit(remote string, hash string) {
	log.Info("New commit detected", "hash", hash)

	repoDir, err := CloneRepo(context.Background(), []string{remote}, hash)
	if err != nil {
		log.Error("failed to clone repo", "hash", hash, "err", err)
		return
	}

	ocuRepoPath := filepath.Join(repoDir, "repo.ocu.star")
	if _, err := os.Stat(ocuRepoPath); os.IsNotExist(err) {
		log.Error("repo.ocu.star not found in repository", "hash", hash, "err", err)
		return
	}

	log.Info("Will work in", "repoDir", repoDir)
	if err := os.Chdir(repoDir); err != nil {
		log.Error("failed to change directory", "hash", hash, "err", err)
		return
	}

	worker, err := NewInRepoWorker(context.Background(), w.Tracker.Ref)
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
