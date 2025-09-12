package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/client/tui"
	"github.com/ocuroot/ocuroot/client/tui/tuiwork"
	"github.com/ocuroot/ocuroot/client/work"
	librelease "github.com/ocuroot/ocuroot/lib/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/store/models"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/trace"
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

		tc, err := getTrackerConfig(ctx, cmd, args)
		if err != nil {
			return err
		}

		force := cmd.Flag("force").Changed

		if !force {
			existingReleases, err := release.GetExistingReleases(ctx, tc)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if len(existingReleases) > 0 {
				fmt.Println(strings.Join(existingReleases, "\n"))
				fmt.Println()
				fmt.Printf("There are already %d releases for this commit (listed above).\nYou can force a new release with the --force flag\n", len(existingReleases))

				return errors.New("release failed")
			}
		}

		cmd.SilenceUsage = true

		workTui := tui.StartWorkTui()
		defer workTui.Cleanup()

		tc.State = tuiwork.WatchForJobUpdates(ctx, tc.State, workTui)

		tracker, environments, err := release.TrackerForNewRelease(ctx, tc)
		if err != nil {
			return err
		}

		if tracker == nil {
			fmt.Println("Registering environments")
			for _, env := range environments {
				// Establishing intent for environment
				intentRef := "@/environment/" + string(env.Name)
				if err := tc.Intent.Set(ctx, intentRef, env); err != nil {
					return err
				}

				tc2 := tc
				tc2.Ref, err = refs.Parse(intentRef)
				if err != nil {
					return err
				}

				worker := &work.Worker{
					Tracker: tc2,
					Tui:     workTui,
				}

				if err := worker.ApplyIntent(ctx, tc2.Ref); err != nil {
					return err
				}

			}
			return nil
		}

		err = tracker.RunToPause(
			ctx,
			tuiwork.TuiLogger(workTui),
		)
		if err != nil {
			return err
		}

		err = workTui.Cleanup()
		if err != nil {
			return err
		}

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

		tc, err := getTrackerConfig(ctx, cmd, args)
		if err != nil {
			return err
		}

		if !tc.Ref.HasRelease() {
			fmt.Println("A release ID or tag must be specified")
			return nil
		}

		cmd.SilenceUsage = true

		workTui := tui.StartWorkTui()
		defer workTui.Cleanup()

		tc.State = tuiwork.WatchForJobUpdates(ctx, tc.State, workTui)

		tracker, err := release.TrackerForExistingRelease(ctx, tc)
		if err != nil {
			if errors.Is(err, refstore.ErrRefNotFound) {
				fmt.Println("The specified release was not found. " + tc.Ref.String())
				return nil
			}
			return err
		}

		err = tracker.RunToPause(
			ctx,
			tuiwork.TuiLogger(workTui),
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

		tc, err := getTrackerConfig(ctx, cmd, args)
		if err != nil {
			return err
		}

		if !tc.Ref.HasRelease() {
			releasesForCommit, err := releasesForCommit(ctx, tc.State, tc.Ref.Repo, tc.Commit)
			if err != nil {
				return fmt.Errorf("failed to get releases for commit: %w", err)
			}

			if len(releasesForCommit) == 0 {
				log.Error("No releases found for commit", "repo", tc.Ref.Repo, "commit", tc.Commit)
				return nil
			}

			sort.Slice(releasesForCommit, func(i, j int) bool {
				return releasesForCommit[i].String() > releasesForCommit[j].String()
			})

			tc.Ref = releasesForCommit[0]
		}

		cmd.SilenceUsage = true

		workTui := tui.StartWorkTui()
		defer workTui.Cleanup()

		tc.State = tuiwork.WatchForJobUpdates(ctx, tc.State, workTui)

		tracker, err := release.TrackerForExistingRelease(ctx, tc)
		if err != nil {
			if errors.Is(err, refstore.ErrRefNotFound) {
				fmt.Println("The specified release was not found. " + tc.Ref.String())
				return nil
			}
			return err
		}

		err = tracker.Retry(
			ctx,
			tuiwork.TuiLogger(workTui),
		)
		if err != nil {
			return err
		}

		return checkFinalReleaseState(ctx, tracker)
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
	ReleaseCmd.AddCommand(NewReleaseCmd)

	NewReleaseCmd.Flags().BoolP("force", "f", false, "Create a new release even if there are existing releases for this commit")

	ReleaseCmd.AddCommand(ContinueReleaseCmd)

	ReleaseCmd.AddCommand(RetryReleaseCmd)

	AddRefFlags(ReleaseCmd, true)

	RootCmd.AddCommand(ReleaseCmd)
}
