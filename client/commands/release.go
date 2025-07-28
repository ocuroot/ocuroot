package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/tree"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/client/state"
	"github.com/ocuroot/ocuroot/client/tui"
	"github.com/ocuroot/ocuroot/client/tui/tuiwork"
	librelease "github.com/ocuroot/ocuroot/lib/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"
	"github.com/spf13/cobra"
)

var ReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Manage releases of a package",
	Long:  `Manage releases of a package, including creating new releases, viewing release state and and resuming paused releases.`,
}

func tuiLogger(tuiWork *tui.WorkTui) func(fnRef refs.Ref, msg sdk.Log) {
	return func(fnRef refs.Ref, msg sdk.Log) {
		wr, err := librelease.WorkRefFromChainRef(fnRef)
		if err != nil {
			log.Error("failed to get work ref", "error", err)
			return
		}
		log.Info("function log", "ref", wr.String(), "msg", msg)

		var task *tuiwork.Task
		t, found := tuiWork.GetTaskByID(wr.String())
		if !found {
			task = &tuiwork.Task{
				TaskID: wr.String(),
				Name:   wr.String(),
				Status: tuiwork.WorkStatusRunning,
			}
		} else {
			task = t.(*tuiwork.Task)
		}
		task.Logs = append(task.Logs, msg.Message)

		tuiWork.UpdateTask(task)
	}
}

func watchForChainUpdates(store refstore.Store, tuiWork *tui.WorkTui) refstore.Store {
	updater := tuiStateChange(store, tuiWork)

	store, err := refstore.ListenToStateChanges(
		func(ctx context.Context, ref string) {
			r, err := refs.Parse(ref)
			if err != nil {
				log.Error("failed to parse ref", "error", err)
				return
			}

			// Ignore deleted refs
			// if err := store.Get(ctx, ref, nil); err == refstore.ErrRefNotFound {
			// 	return
			// }

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

func tuiStateChange(store refstore.Store, tuiWork *tui.WorkTui) func(ref refs.Ref) {
	return func(ref refs.Ref) {
		ctx := context.Background()
		chainRef := librelease.ChainRefFromFunctionRef(ref)
		wr, err := librelease.WorkRefFromChainRef(chainRef)
		if err != nil {
			log.Error("failed to get work ref", "ref", ref.String(), "error", err)
			return
		}
		chainStatus, err := librelease.GetFunctionChainStatusFromFunctions(ctx, store, chainRef)
		if err != nil {
			log.Error("failed to get function chain status", "chainRef", chainRef.String(), "error", err)
			return
		}

		var status tuiwork.WorkStatus
		switch chainStatus {
		case models.SummarizedStatusPending:
			status = tuiwork.WorkStatusPending
		case models.SummarizedStatusRunning:
			status = tuiwork.WorkStatusRunning
		case models.SummarizedStatusComplete:
			status = tuiwork.WorkStatusDone
		case models.SummarizedStatusFailed:
			status = tuiwork.WorkStatusFailed
		default:
			status = tuiwork.WorkStatusDone
		}

		name := strings.Split(chainRef.SubPath, "/")[0]
		if chainRef.SubPathType == refs.SubPathTypeDeploy {
			name = fmt.Sprintf("deploy to %s", name)
		}

		var message string
		if status == tuiwork.WorkStatusDone || status == tuiwork.WorkStatusPending {

			// Get chain outputs and render as a message
			var chainWork models.Work
			if err := store.Get(ctx, chainRef.String(), &chainWork); err != nil {
				log.Error("failed to get chain work", "ref", chainRef.String(), "error", err)
				return
			}

			if status == tuiwork.WorkStatusDone && len(chainWork.Outputs) > 0 {
				outputs := tree.Root("Outputs")
				for k, v := range chainWork.Outputs {
					outputs = outputs.Child(
						tree.Root(
							fmt.Sprintf("%s#output/%s", wr.String(), k),
						).Child(v),
					)
				}
				message += outputs.String()
			}

			if status == tuiwork.WorkStatusPending {
				var fn librelease.FunctionState
				if err := store.Get(ctx, chainWork.Entrypoint.String(), &fn); err != nil {
					log.Error("failed to get function summary", "chainRef", chainRef.String(), "entrypoint", chainWork.Entrypoint.String(), "error", err)
					return
				}

				hasPending := false
				pendingInputs := tree.Root("Pending Inputs")
				for _, v := range fn.Current.Inputs {
					retrieved, err := librelease.RetrieveInput(ctx, store, v)
					if err != nil {
						log.Error("failed to retrieve input", "ref", v.Ref.String(), "error", err)
						return
					}

					if retrieved.Default == nil && retrieved.Value == nil {
						hasPending = true
						pendingInputs = pendingInputs.Child(v.Ref)
					}
				}
				if hasPending {
					message += pendingInputs.String()
				}
			}
		}

		var task *tuiwork.Task
		t, found := tuiWork.GetTaskByID(wr.String())
		if found {
			task = t.(*tuiwork.Task)

			var wasExpected bool

			if task.Status == status {
				wasExpected = true
			}
			if task.Status == tuiwork.WorkStatusPending && status == tuiwork.WorkStatusRunning {
				wasExpected = true
			}
			if task.Status == tuiwork.WorkStatusRunning && status == tuiwork.WorkStatusDone {
				wasExpected = true
			}
			if task.Status == tuiwork.WorkStatusRunning && status == tuiwork.WorkStatusFailed {
				wasExpected = true
			}

			if !wasExpected {
				log.Error("unexpected task status change", "initial", task.Status, "next", status, "id", task.TaskID)
				log.Error("original ref", "ref", ref.String())
			}
			if wasExpected || status == tuiwork.WorkStatusDone || status == tuiwork.WorkStatusFailed {
				task.Status = status
				task.Message = message
			}
		} else {
			task = &tuiwork.Task{
				TaskID:  wr.String(),
				Name:    name,
				Status:  status,
				Message: message,
			}
		}

		if status == tuiwork.WorkStatusRunning {
			task.StartTime = time.Now()
		}
		if status == tuiwork.WorkStatusDone || status == tuiwork.WorkStatusFailed {
			task.EndTime = time.Now()
		}

		tuiWork.UpdateTask(task)
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

		workTui := tui.StartWorkTui(logMode)
		defer workTui.Cleanup()

		tc.Store = watchForChainUpdates(tc.Store, workTui)

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
			tuiLogger(workTui),
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

		workTui := tui.StartWorkTui(logMode)

		tc.Store = watchForChainUpdates(tc.Store, workTui)

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
			tuiLogger(workTui),
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
