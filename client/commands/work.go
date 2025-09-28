package commands

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/client/tui"
	"github.com/ocuroot/ocuroot/client/tui/tuiwork"
	"github.com/ocuroot/ocuroot/client/work"
	"github.com/spf13/cobra"
)

var WorkCmd = &cobra.Command{
	Use:   "work",
	Short: "Manage outstanding work",
	Long:  `Manage outstanding work.`,
}

var WorkContinueCmd = &cobra.Command{
	Use:   "continue",
	Short: "Continue outstanding work",
	Long:  `Continue outstanding work against the current commit.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		ref, err := GetRef(cmd, args)
		if err != nil {
			return fmt.Errorf("failed to get ref: %w", err)
		}

		dryRun := cmd.Flag("dryrun").Changed
		cmd.SilenceUsage = true

		worker, err := work.NewWorker(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to create worker: %w", err)
		}
		defer worker.Cleanup()

		todo, err := worker.ReadyRuns(ctx, work.IndentifyWorkRequest{
			GitFilter: work.GitFilterCurrentCommitOnly,
		})
		if err != nil {
			return fmt.Errorf("failed to identify work: %w", err)
		}
		reconcilableDeployments, err := worker.ReconcilableDeployments(ctx, work.IndentifyWorkRequest{
			GitFilter: work.GitFilterCurrentCommitOnly,
		})
		if err != nil {
			return fmt.Errorf("failed to get reconcilable deployments: %w", err)
		}
		todo = append(todo, reconcilableDeployments...)

		log.Info("Identified work", "todo", toJSON(todo))

		if dryRun {
			worker.Cleanup()

			todoJSON, err := json.MarshalIndent(todo, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal todo: %w", err)
			}
			fmt.Println(string(todoJSON))
			return nil
		}

		if err := worker.ExecuteWork(ctx, todo); err != nil {
			return err
		}

		return nil
	},
}

var WorkAnyCommand = &cobra.Command{
	Use:   "any",
	Short: "Pick up any outstanding work",
	Long: `Pick up any outstanding work based on the contents of the state store.

Will start by running any release work (equivalent to 'ocuroot work continue'), then
any ops ('ocuroot work ops'), then sync any intent ('ocuroot state diff | xargs -r -n1 ocuroot state apply'),
finally it will trigger work for other commits ('ocuroot work trigger').
	`,
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		comprehensive := cmd.Flag("comprehensive").Changed
		dryRun := cmd.Flag("dryrun").Changed

		if comprehensive && dryRun {
			return fmt.Errorf("--comprehensive and --dryrun are mutually exclusive")
		}

		ref, err := GetRef(cmd, args)
		if err != nil {
			return fmt.Errorf("failed to get ref: %w", err)
		}

		cmd.SilenceUsage = true

		worker, err := work.NewWorker(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to create worker: %w", err)
		}
		defer worker.Cleanup()

		for {
			todo, err := worker.IdentifyWork(ctx, work.IndentifyWorkRequest{
				GitFilter: work.GitFilterCurrentCommitOnly,
			})
			if err != nil {
				return fmt.Errorf("failed to identify work: %w", err)
			}

			log.Info("Identified work", "todo", toJSON(todo))
			if len(todo) == 0 {
				return nil
			}

			if dryRun {
				worker.Cleanup()

				todoJSON, err := json.MarshalIndent(todo, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal todo: %w", err)
				}
				fmt.Println(string(todoJSON))
				return nil
			}

			if err := worker.ExecuteWork(ctx, todo); err != nil {
				return err
			}

			if !comprehensive {
				break
			}
		}

		log.Info("Starting trigger work")
		todo, err := worker.IdentifyWork(ctx, work.IndentifyWorkRequest{})
		if err != nil {
			return fmt.Errorf("failed to identify work: %w", err)
		}
		if err := worker.TriggerWork(ctx, todo); err != nil {
			return err
		}
		return nil
	},
}

var WorkTriggerCommand = &cobra.Command{
	Use:   "trigger",
	Short: "Trigger outstanding runs in the state store",
	Long:  `Trigger outstanding runs in the state store.`,
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		ref, err := GetRef(cmd, args)
		if err != nil {
			return fmt.Errorf("failed to get ref: %w", err)
		}

		w, err := work.NewWorker(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to create worker: %w", err)
		}
		w.Cleanup()

		workTui := tui.StartWorkTui()
		defer workTui.Cleanup()

		tc := release.TrackerConfig{
			State: w.Tracker.State,
		}
		tc.State = tuiwork.WatchForStateUpdates(ctx, tc.State, workTui)

		worker := &work.Worker{
			Tracker: tc,
			Tui:     workTui,
		}

		dryRun := cmd.Flag("dryrun").Changed
		intent := cmd.Flag("intent").Changed
		if intent {
			if err := worker.TriggerAll(ctx); err != nil {
				return err
			}
			return nil
		}

		todo, err := worker.IdentifyWork(ctx, work.IndentifyWorkRequest{})
		if err != nil {
			return fmt.Errorf("failed to identify work: %w", err)
		}

		if dryRun {
			todoJSON, err := json.MarshalIndent(todo, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal todo: %w", err)
			}
			fmt.Println(string(todoJSON))
			return nil
		}

		if err := worker.TriggerWork(ctx, todo); err != nil {
			return err
		}
		return nil
	},
}

var WorkOpsCmd = &cobra.Command{
	Use:   "ops",
	Short: "Run scheduled ops",
	Long:  `Run scheduled operations against this commit.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		dryRun := cmd.Flag("dryrun").Changed

		ref, err := GetRef(cmd, args)
		if err != nil {
			return fmt.Errorf("failed to get ref: %w", err)
		}

		cmd.SilenceUsage = true

		worker, err := work.NewWorker(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to create worker: %w", err)
		}
		defer worker.Cleanup()

		todo, err := worker.Ops(ctx, work.IndentifyWorkRequest{
			GitFilter: work.GitFilterCurrentCommitOnly,
		})
		if err != nil {
			return fmt.Errorf("failed to identify work: %w", err)
		}

		log.Info("Identified work", "todo", toJSON(todo))

		if dryRun {
			worker.Cleanup()

			todoJSON, err := json.MarshalIndent(todo, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal todo: %w", err)
			}
			fmt.Println(string(todoJSON))
			return nil
		}

		if err := worker.ExecuteWork(ctx, todo); err != nil {
			return fmt.Errorf("failed to execute work: %w", err)
		}

		return nil
	},
}

var WorkCascadeCommand = &cobra.Command{
	Use:   "cascade",
	Short: "Pick up any outstanding work",
	Long: `Pick up any outstanding work based on the contents of the state store.

Will start by running any release work (equivalent to 'ocuroot work continue'), then
any ops ('ocuroot work ops'), then sync any intent ('ocuroot state diff | xargs -r -n1 ocuroot state apply'),
finally it will trigger work for other commits ('ocuroot work trigger').
	`,
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		dryRun := cmd.Flag("dryrun").Changed

		ref, err := GetRef(cmd, args)
		if err != nil {
			return fmt.Errorf("failed to get ref: %w", err)
		}

		cmd.SilenceUsage = true

		worker, err := work.NewWorker(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to create worker: %w", err)
		}
		defer worker.Cleanup()

		for {
			todo, err := worker.IdentifyWork(ctx, work.IndentifyWorkRequest{})
			if err != nil {
				return fmt.Errorf("failed to identify work: %w", err)
			}

			log.Info("Identified work", "todo", toJSON(todo))
			if len(todo) == 0 {
				return nil
			}

			if dryRun {
				worker.Cleanup()

				todoJSON, err := json.MarshalIndent(todo, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal todo: %w", err)
				}
				fmt.Println(string(todoJSON))
				return nil
			}

			if err := worker.ExecuteWorkInCleanWorktrees(ctx, todo); err != nil {
				return err
			}
		}

		return nil
	},
}

func toJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}

func init() {
	RootCmd.AddCommand(WorkCmd)

	WorkContinueCmd.Flags().BoolP("dryrun", "d", false, "List refs for work that would be triggered")
	WorkCmd.AddCommand(WorkContinueCmd)

	WorkTriggerCommand.Flags().BoolP("dryrun", "d", false, "List refs for work that would be triggered")
	WorkTriggerCommand.Flags().BoolP("intent", "i", false, "Trigger intents instead of deployments")
	WorkCmd.AddCommand(WorkTriggerCommand)

	WorkOpsCmd.Flags().BoolP("dryrun", "d", false, "List refs for work that would be triggered")
	WorkCmd.AddCommand(WorkOpsCmd)

	WorkAnyCommand.Flags().Bool("comprehensive", false, "Run in comprehensive mode, which will continue requesting work until this commit is stable")
	WorkAnyCommand.Flags().BoolP("dryrun", "d", false, "List refs for work that would be triggered. Will only list the first set of work so is incompatible with --comprehensive")
	WorkCmd.AddCommand(WorkAnyCommand)

	WorkCascadeCommand.Flags().BoolP("dryrun", "d", false, "List refs for work that would be triggered")
	WorkCmd.AddCommand(WorkCascadeCommand)
}
