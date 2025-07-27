package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
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
		ctx := context.Background()
		chainRef := librelease.ChainRefFromFunctionRef(ref)
		wr, err := librelease.WorkRefFromChainRef(chainRef)
		if err != nil {
			log.Error("failed to get work ref", "error", err)
			return
		}
		chainStatus, err := librelease.GetFunctionChainStatusFromFunctions(ctx, store, chainRef)
		if err != nil {
			log.Error("failed to get function chain status", "error", err)
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

		var message string
		if status == tui.WorkStatusDone || status == tui.WorkStatusPending {

			// Get chain outputs and render as a message
			var chainWork models.Work
			if err := store.Get(ctx, chainRef.String(), &chainWork); err != nil {
				log.Error("failed to get chain work", "error", err)
				return
			}

			if status == tui.WorkStatusDone && len(chainWork.Outputs) > 0 {
				message = "Outputs\n"
				var lines []string
				for k, v := range chainWork.Outputs {
					lines = append(lines, fmt.Sprintf("* %s#output/%s\n\t%v", wr.String(), k, v))
				}
				message += strings.Join(lines, "\n")
			}

			if status == tui.WorkStatusPending {
				var fn librelease.FunctionState
				if err := store.Get(ctx, chainWork.Entrypoint.String(), &fn); err != nil {
					log.Error("failed to get function summary", "error", err)
					return
				}
				var lines []string
				for _, v := range fn.Current.Inputs {
					retrieved, err := librelease.RetrieveInput(ctx, store, v)
					if err != nil {
						log.Error("failed to retrieve input", "error", err)
						return
					}
					if retrieved.Default == nil && retrieved.Value == nil {
						if message == "" {
							message = "Pending Inputs\n"
						}
						lines = append(lines, fmt.Sprintf("* %s", v.Ref))
					}
				}
				message += strings.Join(lines, "\n")
			}
		}

		event := tui.TaskEvent{
			ID:      wr.String(),
			Name:    name,
			Status:  status,
			Message: message,
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

	if status == models.SummarizedStatusFailed {
		return fmt.Errorf("release failed")
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
