package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ocuroot/ocuroot/client"
	"github.com/ocuroot/ocuroot/client/tui/tuiwork"
	"github.com/ocuroot/ocuroot/client/work"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/spf13/cobra"
)

var PushCmd = &cobra.Command{
	Use:   "push",
	Short: "Handle a push to a git repo",
	Long:  `Handle a push to a git repo`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		wd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoInfo, err := client.GetRepoInfo(wd)
		if err != nil {
			return err
		}

		if repoInfo.IsSource {
			return pushSourceRepo(ctx, repoInfo)
		}

		return pushStateRepo(ctx, repoInfo)
	},
}

func pushSourceRepo(ctx context.Context, repoInfo client.RepoInfo) error {
	if !repoInfo.IsSource {
		panic("not a source repo")
	}

	files, err := repoInfo.GetReleaseConfigFiles()
	if err != nil {
		return err
	}

	for _, file := range files {
		if err := releaseConfigFile(ctx, repoInfo, file); err != nil {
			return err
		}
	}

	return nil
}

func releaseConfigFile(ctx context.Context, repoInfo client.RepoInfo, file string) error {
	subPath, err := filepath.Rel(repoInfo.Root, file)
	if err != nil {
		return err
	}

	ref, err := refs.Parse(fmt.Sprintf("./-/%v", subPath))
	if err != nil {
		return err
	}

	worker, err := work.NewWorker(ctx, ref)
	if err != nil {
		return err
	}
	defer worker.Cleanup()

	tc := worker.Tracker

	tracker, environments, err := worker.TrackerForNewRelease(ctx)
	if err != nil {
		return err
	}

	if tracker == nil {
		for _, env := range environments {
			// Establishing intent for environment
			intentRef := "@/environment/" + string(env.Name)
			if err := tc.Intent.Set(ctx, intentRef, env); err != nil {
				return err
			}

			pr, err := refs.Parse(intentRef)
			if err != nil {
				return err
			}

			if err := worker.ApplyIntent(ctx, pr); err != nil {
				return err
			}
		}

		if err := worker.Cascade(ctx); err != nil {
			return err
		}
		return nil
	}

	err = tracker.RunToPause(
		ctx,
		tuiwork.TuiLogger(worker.Tui),
	)
	if err != nil {
		return err
	}

	if err := worker.Cascade(ctx); err != nil {
		return err
	}

	worker.Cleanup()

	return nil
}

func pushStateRepo(ctx context.Context, repoInfo client.RepoInfo) error {
	if repoInfo.IsSource {
		panic("not a state repo")
	}

	return nil
}

func init() {
	RootCmd.AddCommand(PushCmd)
}
