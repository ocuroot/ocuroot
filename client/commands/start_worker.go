package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/ocuroot/ocuroot/client/work"
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
		if workerDev {
			return devWorker(ctx)
		} else if workerBranch == "" {
			return fmt.Errorf("branch not specified")
		}
		return fmt.Errorf("non-dev mode not yet implemented; use --dev flag for local repo")

	},
}

func devWorker(ctx context.Context) error {
	ref, err := GetRef(nil, nil)
	if err != nil {
		return fmt.Errorf("failed to get ref: %w", err)
	}

	worker, err := work.NewInRepoWorker(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to create worker: %w", err)
	}
	defer worker.Cleanup()

	if err := worker.Poll(ctx); err != nil {
		return fmt.Errorf("failed to poll: %w", err)
	}

	return nil
}

func init() {
	WorkerCmd.Flags().BoolVar(&workerDev, "dev", false, "Use current directory as repository. Will monitor for commits (every 1s) and release based on changed files. Should not be used in production.")
	WorkerCmd.Flags().DurationVar(&workerInterval, "interval", time.Minute, "Polling interval (e.g., 30s, 1m, 5m). Ignored in dev mode.")
	WorkerCmd.Flags().StringVar(&workerBranch, "branch", "", "Branch to poll, in dev mode defaults to the currently checked out branch")

	StartCmd.AddCommand(WorkerCmd)
}
