package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client"
	"github.com/ocuroot/ocuroot/client/work"
	"github.com/ocuroot/ocuroot/git"
	"github.com/spf13/cobra"
)

var (
	workerDev      bool
	workerInterval time.Duration
	workerBranch   string
)

var WorkerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Start a worker that polls for new commits",
	Long:  `Start a worker that polls a git repository for new commits and processes them.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		// Determine repository URL
		var repoURL string
		if workerDev {
			// Use current directory as file:// URL
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			repoInfo, err := client.GetRepoInfo(cwd)
			if err != nil {
				return fmt.Errorf("failed to get repo info: %w", err)
			}

			if workerBranch == "" {
				workerBranch = fmt.Sprintf("refs/heads/%v", repoInfo.Branch)
			}
			repoURL = "file://" + cwd
			workerInterval = time.Second // Can afford to poll very frequently when running locally
			log.Info("Using local repository", "path", cwd)
		} else if workerBranch == "" {
			return fmt.Errorf("branch not specified")
		} else {
			return fmt.Errorf("non-dev mode not yet implemented; use --dev flag for local repo")
		}

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
			commitHandler,
			ticker.C,
		)
		if err != nil && err != context.Canceled {
			return fmt.Errorf("polling error: %w", err)
		}

		log.Info("Worker stopped")
		return nil
	},
}

func commitHandler(remote string, hash string) {
	log.Info("New commit detected", "hash", hash)

	repoDir, err := work.CloneRepo(context.Background(), []string{remote}, hash)
	if err != nil {
		log.Error("failed to clone repo", "hash", hash, "err", err)
		return
	}

	ref, err := GetRef(nil, nil)
	if err != nil {
		log.Error("failed to get ref", "hash", hash, "err", err)
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

	worker, err := work.NewInRepoWorker(context.Background(), ref)
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

func init() {
	WorkerCmd.Flags().BoolVar(&workerDev, "dev", false, "Use current directory as repository. Will monitor for commits (every 1s) and release based on changed files. Should not be used in production.")
	WorkerCmd.Flags().DurationVar(&workerInterval, "interval", time.Minute, "Polling interval (e.g., 30s, 1m, 5m). Ignored in dev mode.")
	WorkerCmd.Flags().StringVar(&workerBranch, "branch", "", "Branch to poll, in dev mode defaults to the currently checked out branch")

	StartCmd.AddCommand(WorkerCmd)
}
