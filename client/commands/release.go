package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client"
	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/client/state"
	"github.com/ocuroot/ocuroot/client/tui"
	librelease "github.com/ocuroot/ocuroot/lib/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"
	"github.com/spf13/cobra"

	tea "github.com/charmbracelet/bubbletea"
)

var ReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Manage releases of a package",
	Long:  `Manage releases of a package, including creating new releases, viewing release state and and resuming paused releases.`,
}

func tuiLogger(tuiUpdate func(tea.Msg)) func(fnRef refs.Ref, msg sdk.Log) {
	return func(fnRef refs.Ref, msg sdk.Log) {
		chainRef := librelease.ChainRefFromFunctionRef(fnRef)
		log.Info("function log", "ref", fnRef, "msg", msg)
		tuiUpdate(tui.FunctionLogToEvent(chainRef, msg))
	}
}

func watchForChainUpdates(store refstore.Store, tuiUpdate func(tea.Msg)) refstore.Store {
	updater := tuiStateChange(store, tuiUpdate)

	store, err := refstore.ListenToStateChanges(
		func(ctx context.Context, ref string) {
			r, err := refs.Parse(ref)
			if err != nil {
				log.Error("failed to parse ref", "error", err)
				return
			}
			updater(r)
		},
		store,
		"**/{call,deploy}/*/*/status/*",
	)
	if err != nil {
		log.Error("failed to listen to state changes", "error", err)
		return store
	}

	return store
}

func tuiStateChange(store refstore.Store, tuiUpdate func(tea.Msg)) func(ref refs.Ref) {
	return func(ref refs.Ref) {
		chainRef := librelease.ChainRefFromFunctionRef(ref)
		chainStatus, err := librelease.GetFunctionChainStatusFromFunctions(context.Background(), store, chainRef)
		if err != nil {
			log.Error("failed to get function chain status", "error", err)
			return
		}

		// Ignore pending chains for the sake of tracking active work
		if chainStatus == models.SummarizedStatusPending {
			return
		}

		var status tui.WorkStatus
		switch chainStatus {
		case models.SummarizedStatusPending:
			status = tui.WorkStatusPending
		case models.SummarizedStatusRunning:
			status = tui.WorkStatusRunning
		case models.SummarizedStatusComplete:
			status = tui.WorkStatusDone
		case models.SummarizedStatusFailed:
			status = tui.WorkStatusFailed
		default:
			status = tui.WorkStatusDone
		}

		name := strings.Split(chainRef.SubPath, "/")[0]
		if chainRef.SubPathType == refs.SubPathTypeDeploy {
			name = fmt.Sprintf("deploy to %s", name)
		}

		event := tui.TaskEvent{
			ID:     chainRef.String(),
			Name:   name,
			Status: status,
		}

		tuiUpdate(event)
	}
}

var NewReleaseCmd = &cobra.Command{
	Use:   "new [package ref]",
	Short: "Create a new release",
	Long: `Create a new release of a package based on the current state of the source repo.
	`,
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, span := tracer.Start(cmd.Context(), "ocuroot release new")
		defer span.End()

		tc, err := getTrackerConfig(cmd, args)
		if err != nil {
			return err
		}

		logMode := cmd.Flag("logmode").Changed
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

				return nil
			}
		}

		cmd.SilenceUsage = true

		tuiUpdate, tuiCleanup, err := tui.StartWorkTui(logMode)
		if err != nil {
			return err
		}
		defer tuiCleanup()

		tc.Store = watchForChainUpdates(tc.Store, tuiUpdate)

		tracker, environments, err := release.TrackerForNewRelease(ctx, tc)
		if err != nil {
			return err
		}

		if tracker == nil {
			fmt.Println("Registering environments")
			for _, env := range environments {
				// Establishing intent for environment
				intentRef := "+/environment/" + string(env.Name)
				if err := tc.Store.Set(ctx, intentRef, env); err != nil {
					return err
				}

				tc2 := tc
				tc2.Ref, err = refs.Parse(intentRef)
				if err != nil {
					return err
				}

				if err := state.ApplyIntent(ctx, tc2); err != nil {
					return err
				}

			}
			return nil
		}

		err = tracker.RunToPause(
			ctx,
			tuiLogger(tuiUpdate),
		)
		if err != nil {
			return err
		}

		err = tuiCleanup()
		if err != nil {
			return err
		}

		return reportOnFinalReleaseState(ctx, tracker)
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

		tc, err := getTrackerConfig(cmd, args)
		if err != nil {
			return err
		}

		logMode := cmd.Flag("logmode").Changed

		if tc.Ref.ReleaseOrIntent.Type != refs.Release {
			fmt.Println("A release ID or tag must be specified")
			return nil
		}

		cmd.SilenceUsage = true

		tuiUpdate, tuiCleanup, err := tui.StartWorkTui(logMode)
		if err != nil {
			return err
		}
		defer tuiCleanup()

		tc.Store = watchForChainUpdates(tc.Store, tuiUpdate)

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
			tuiLogger(tuiUpdate),
		)
		if err != nil {
			return err
		}

		err = tuiCleanup()
		if err != nil {
			return err
		}

		return reportOnFinalReleaseState(ctx, tracker)
	},
}

func reportOnFinalReleaseState(
	ctx context.Context,
	tracker *librelease.ReleaseTracker,
) error {
	status, err := tracker.ReleaseStatus(ctx)
	if err != nil {
		return err
	}

	if status == models.SummarizedStatusFailed {
		return fmt.Errorf("release failed")
	}
	if status == models.SummarizedStatusCancelled {
		fmt.Println("Release cancelled")
		return nil
	}
	if status == models.SummarizedStatusComplete {
		fmt.Println("Release complete")
		return nil
	}

	tags, err := tracker.GetTags(ctx)
	if err != nil {
		return err
	}

	filteredNextFunctions, err := tracker.FilteredNextFunctions(ctx)
	if err != nil {
		return err
	}

	if len(filteredNextFunctions) != 0 {
		fmt.Println("Release ready to continue")
		fmt.Println()
		fmt.Println("You can continue the release with the below command:")
		fmt.Printf("  %v release continue %v\n", client.Command(), tracker.ReleaseRef)
		fmt.Println()
		if len(tags) > 0 {
			fmt.Println("Or by using tags:")
			tref := tracker.ReleaseRef
			for _, tag := range tags {
				tref.ReleaseOrIntent.Value = tag
				fmt.Printf("  %v release continue %s\n", client.Command(), tref)
			}
		}
		return nil
	}

	nextFunctions, err := tracker.UnfilteredNextFunctions(ctx)
	if err != nil {
		return err
	}

	if len(nextFunctions) > 0 {
		fmt.Println()
		fmt.Println("The following functions are blocked waiting for inputs:")
		for fr, fn := range nextFunctions {
			fmt.Println(fr)
			missingInputs, err := tracker.PopulateInputs(ctx, fr, fn)
			if err != nil {
				return err
			}
			for _, input := range missingInputs {
				fmt.Printf("\t%v\n", input.Ref)
				if input.Doc != nil {
					fmt.Printf("\t%v\n", *input.Doc)
				}
				fmt.Println()
			}
		}
		fmt.Println()
		fmt.Println("Once the above inputs have been satisfied, you can resume this release by running:")
		fmt.Printf("  %v release continue %v\n", client.Command(), tracker.ReleaseRef)
		fmt.Println()
		if len(tags) > 0 {
			fmt.Println("Or by using tags:")
			tref := tracker.ReleaseRef
			for _, tag := range tags {
				tref.ReleaseOrIntent.Value = tag
				fmt.Printf("  %v release continue %v\n", client.Command(), tref)
			}
		}
		return nil
	}
	return nil
}

func init() {
	ReleaseCmd.AddCommand(NewReleaseCmd)

	NewReleaseCmd.Flags().BoolP("logmode", "l", false, "Enable log mode when initializing the TUI")
	NewReleaseCmd.Flags().BoolP("force", "f", false, "Create a new release even if there are existing releases for this commit")

	ReleaseCmd.AddCommand(ContinueReleaseCmd)

	ContinueReleaseCmd.Flags().BoolP("logmode", "l", false, "Enable log mode when initializing the TUI")

	AddRefFlags(ReleaseCmd, true)

	RootCmd.AddCommand(ReleaseCmd)
}
