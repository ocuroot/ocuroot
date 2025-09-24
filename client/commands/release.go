package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/local"
	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/client/tui/tuiwork"
	"github.com/ocuroot/ocuroot/client/work"
	librelease "github.com/ocuroot/ocuroot/lib/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/store/models"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/trace"
	"go.starlark.net/starlark"
)

var ReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Manage releases of a package",
	Long:  `Manage releases of a package, including creating new releases, viewing release state and and resuming paused releases.`,
}

var NewReleaseCmd = &cobra.Command{
	Use:   "new [package ref]",
	Short: "Create a new release",
	Long: `Create a new release of a package based on the current state of the source repo.
	`,
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, span := tracer.Start(
			cmd.Context(),
			"ocuroot release new",
			trace.WithNewRoot(),
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		force := cmd.Flag("force").Changed
		cascade := cmd.Flag("cascade").Changed

		ref, err := GetRef(cmd, args)
		if err != nil {
			return fmt.Errorf("failed to get ref: %w", err)
		}

		cmd.SilenceUsage = true

		worker, err := work.NewWorker(ctx, ref)
		if err != nil {
			return err
		}
		defer worker.Cleanup()

		tc := worker.Tracker

		if !force {
			existingReleases, err := release.GetExistingReleases(ctx, tc)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if len(existingReleases) > 0 {
				worker.Cleanup()

				fmt.Println(strings.Join(existingReleases, "\n"))
				fmt.Println()
				fmt.Printf("There are already %d releases for this commit (listed above).\nYou can force a new release with the --force flag\n", len(existingReleases))

				return errors.New("release failed")
			}
		}

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
			return nil
		}

		err = tracker.RunToPause(
			ctx,
			tuiwork.TuiLogger(worker.Tui),
		)
		if err != nil {
			return err
		}

		if cascade {
			if err := worker.Cascade(ctx); err != nil {
				return err
			}
		}

		worker.Cleanup()

		return checkFinalReleaseState(ctx, tracker)
	},
}

var ContinueReleaseCmd = &cobra.Command{
	Use:   "continue [release ref]",
	Short: "Continue a release to allow it to progress.",
	Long:  `Continue a release to allow it to progress.`,
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, span := tracer.Start(cmd.Context(), "ocuroot release continue")
		defer span.End()

		ref, err := GetRef(cmd, args)
		if err != nil {
			return err
		}

		if !ref.HasRelease() {
			fmt.Println("A release ID or tag must be specified")
			return nil
		}

		cmd.SilenceUsage = true

		worker, err := work.NewWorker(ctx, ref)
		if err != nil {
			return err
		}
		defer worker.Cleanup()

		tracker, err := worker.TrackerForExistingRelease(ctx)
		if err != nil {
			if errors.Is(err, refstore.ErrRefNotFound) {
				fmt.Println("The specified release was not found. " + ref.String())
				return nil
			}
			return err
		}

		err = tracker.RunToPause(
			ctx,
			tuiwork.TuiLogger(worker.Tui),
		)
		if err != nil {
			return err
		}

		return checkFinalReleaseState(ctx, tracker)
	},
}

var RetryReleaseCmd = &cobra.Command{
	Use:   "retry [release ref]",
	Short: "Retry a failed release",
	Long:  `Retry a failed release.`,
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, span := tracer.Start(cmd.Context(), "ocuroot release retry")
		defer span.End()

		ref, err := GetRef(cmd, args)
		if err != nil {
			return err
		}

		cmd.SilenceUsage = true

		worker, err := work.NewWorker(ctx, ref)
		if err != nil {
			return err
		}
		defer worker.Cleanup()

		if !ref.HasRelease() {
			releasesForCommit, err := releasesForCommit(ctx, worker.Tracker.State, worker.Tracker.Ref.Repo, worker.Tracker.Commit)
			if err != nil {
				return fmt.Errorf("failed to get releases for commit: %w", err)
			}

			if len(releasesForCommit) == 0 {
				log.Error("No releases found for commit", "repo", worker.Tracker.Ref.Repo, "commit", worker.Tracker.Commit)
				return nil
			}

			sort.Slice(releasesForCommit, func(i, j int) bool {
				return releasesForCommit[i].String() > releasesForCommit[j].String()
			})

			worker.Tracker.Ref = releasesForCommit[0]
		}

		tracker, err := worker.TrackerForExistingRelease(ctx)
		if err != nil {
			if errors.Is(err, refstore.ErrRefNotFound) {
				fmt.Println("The specified release was not found. " + worker.Tracker.Ref.String())
				return nil
			}
			return err
		}

		err = tracker.Retry(
			ctx,
			tuiwork.TuiLogger(worker.Tui),
		)
		if err != nil {
			return err
		}

		return checkFinalReleaseState(ctx, tracker)
	},
}

var LintReleaseCmd = &cobra.Command{
	Use:   "lint [package-file]",
	Short: "Lint a config file containing a release",
	Long: `Execute a config file to check for errors.
	
This involves loading state for environment lists, so the state and intent stores must be correctly configured.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		ref, err := GetRef(cmd, args)
		if err != nil {
			return err
		}

		cmd.SilenceUsage = true

		worker, err := work.NewWorker(ctx, ref)
		if err != nil {
			return err
		}
		defer worker.Cleanup()

		backend, _ := release.NewBackend(worker.Tracker)
		config, err := local.ExecutePackageWithLogging(ctx, worker.Tracker.RepoPath, worker.Tracker.Ref, backend, func(thread *starlark.Thread, msg string) {
			fmt.Println(msg)
		})
		if err != nil {
			return fmt.Errorf("failed to load config %w", err)
		}

		worker.Cleanup()

		if config.Package == nil {
			fmt.Println("No release configured")
		} else {
			for _, phase := range config.Package.Phases {
				if len(phase.Tasks) > 1 || len(phase.Name) > 0 {
					fmt.Printf("Phase %q: %v tasks\n", phase.Name, len(phase.Tasks))
				} else if len(phase.Tasks) == 1 {
					t := phase.Tasks[0]
					if t.Task != nil {
						fmt.Printf("Task: %q\n", t.Task.Name)
					}
					if t.Deployment != nil {
						fmt.Printf("Deployment to %q\n", t.Deployment.Environment)
					}
				}
			}
		}
		fmt.Println("Config file evaluated successfully")

		return nil
	},
}

func checkFinalReleaseState(
	ctx context.Context,
	tracker *librelease.ReleaseTracker,
) error {
	status, err := tracker.ReleaseStatus(ctx)
	if err != nil {
		return err
	}

	if status == models.StatusFailed {
		return fmt.Errorf("release failed")
	}

	return nil
}

func init() {
	NewReleaseCmd.Flags().BoolP("force", "f", false, "Create a new release even if there are existing releases for this commit")
	NewReleaseCmd.Flags().Bool("cascade", false, "Create a new release and cascade follow on work for dependant releases")

	ReleaseCmd.AddCommand(NewReleaseCmd)

	ReleaseCmd.AddCommand(ContinueReleaseCmd)
	ReleaseCmd.AddCommand(RetryReleaseCmd)
	ReleaseCmd.AddCommand(LintReleaseCmd)

	AddRefFlags(ReleaseCmd, true)

	RootCmd.AddCommand(ReleaseCmd)
}
