package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/client/state"
	"github.com/ocuroot/ocuroot/client/work"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/spf13/cobra"
)

var StateCmd = &cobra.Command{
	Use:   "state",
	Short: "Commands for working with Ocuroot state",
	Long:  `Commands for working with Ocuroot state.`,
}

var StateGetCmd = &cobra.Command{
	Use:   "get [ref]",
	Short: "Get state for a specific ref.",
	Long:  `Get state for a specific ref.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		ref, err := GetRef(cmd, args)
		if err != nil {
			log.Error("Failed to get ref", "error", err)
			return fmt.Errorf("failed to get ref: %w", err)
		}

		cmd.SilenceUsage = true

		w, err := work.NewWorker(ctx, ref)
		if err != nil {
			log.Error("Failed to create worker", "error", err)
			return fmt.Errorf("failed to create worker: %w", err)
		}
		w.Cleanup()

		var v any
		err = w.Tracker.State.Get(cmd.Context(), w.Tracker.Ref.String(), &v)
		if err != nil {
			log.Error("Failed to get state", "ref", w.Tracker.Ref.String(), "error", err)
			return fmt.Errorf("failed to get state: %w", err)
		}

		jv, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			log.Error("Failed to marshal state", "error", err)
			return fmt.Errorf("failed to marshal state: %w", err)
		}

		log.Info("Returning state", "value", string(jv))
		fmt.Println(string(jv))
		return nil
	},
}

var StateMatchCmd = &cobra.Command{
	Use:   "match [glob]",
	Short: "List refs matching the specified glob.",
	Long:  `List refs matching the specified glob.`,
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

		glob := ""
		if len(args) > 0 {
			glob = args[0]
		}

		noLinks, err := cmd.Flags().GetBool("no-links")
		if err != nil {
			return fmt.Errorf("failed to get no-links flag: %w", err)
		}

		cmd.SilenceUsage = true

		refs, err := w.Tracker.State.MatchOptions(cmd.Context(), refstore.MatchOptions{
			NoLinks: noLinks,
		}, glob)
		if err != nil {
			return fmt.Errorf("failed to match refs: %w", err)
		}

		for _, ref := range refs {
			fmt.Println(ref)
		}

		return nil
	},
}

var StateDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Diff intent with current state.",
	Long:  `Diff intent with current state.`,
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

		cmd.SilenceUsage = true

		diffs, err := w.Diff(ctx, work.IndentifyWorkRequest{
			GitFilter: work.GitFilterCurrentCommitOnly,
		})
		if err != nil {
			return fmt.Errorf("failed to diff: %w", err)
		}

		for _, diff := range diffs {
			fmt.Println(diff.Ref.String())
		}

		return nil
	},
}

var StateDeleteIntentCmd = &cobra.Command{
	Use:   "delete [intent-ref]",
	Short: "Delete intent",
	Long:  `Delete an intent from the current state.`,
	Args:  cobra.ExactArgs(1),
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

		cmd.SilenceUsage = true

		intent := w.Tracker.Intent

		cmd.SilenceUsage = true

		if err := intent.Delete(ctx, w.Tracker.Ref.String()); err != nil {
			return fmt.Errorf("failed to delete intent: %w", err)
		}

		return nil
	},
}

var StateSetIntentCmd = &cobra.Command{
	Use:   "set [intent-ref] [value]",
	Short: "Set intent",
	Long: `Set the value of an intent.

Set value to '-' to pass the value from stdin.
`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		format, err := cmd.Flags().GetString("format")
		if err != nil {
			return fmt.Errorf("failed to get format flag: %w", err)
		}
		switch format {
		case "json", "starlark", "string":
		default:
			return fmt.Errorf("unsupported format: %s", format)
		}

		ref, err := GetRef(cmd, args)
		if err != nil {
			return fmt.Errorf("failed to get ref: %w", err)
		}

		w, err := work.NewWorker(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to create worker: %w", err)
		}
		w.Cleanup()

		cmd.SilenceUsage = true

		intent := w.Tracker.Intent

		// Subsequent failures should not output usage information.
		cmd.SilenceUsage = true

		valueStr := args[1]

		var value any
		if valueStr == "-" {
			return fmt.Errorf("reading from stdin is not implemented")
		}

		if format == "string" {
			value = valueStr
		}
		if format == "json" {
			err = json.Unmarshal([]byte(valueStr), &value)
			if err != nil {
				return fmt.Errorf("failed to unmarshal value: %w", err)
			}
		}
		if format == "starlark" {
			value, err = evalValue(ctx, valueStr)
			if err != nil {
				return fmt.Errorf("failed to evaluate value: %w", err)
			}
		}

		if err := intent.Set(ctx, w.Tracker.Ref.String(), value); err != nil {
			return fmt.Errorf("failed to set intent: %w", err)
		}

		return nil
	},
}

func evalValue(ctx context.Context, value string) (any, error) {
	backend, _ := release.NewBackend(release.TrackerConfig{
		Ref: refs.Ref{},
	})
	return sdk.Eval(ctx, backend, "0.3.0", value)
}

var StateApplyIntentCmd = &cobra.Command{
	Use:   "apply [intent-ref]",
	Short: "Apply intent",
	Long:  `Apply an intent to the current state. May require triggering functions.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		ref, err := GetRef(cmd, args)
		if err != nil {
			return fmt.Errorf("failed to get ref: %w", err)
		}
		cascade := cmd.Flags().Changed("cascade")

		cmd.SilenceUsage = true

		worker, err := work.NewWorker(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to create worker: %w", err)
		}
		defer worker.Cleanup()

		if err := worker.ApplyIntent(ctx, worker.Tracker.Ref); err != nil {
			return fmt.Errorf("failed to apply intent: %w", err)
		}

		if cascade {
			if err := worker.Cascade(ctx); err != nil {
				return err
			}
		}

		return nil
	},
}

var StateViewCmd = &cobra.Command{
	Use:   "view",
	Short: "View state in a web browser.",
	Long:  `View state in a web browser.`,
	Args:  cobra.ExactArgs(0),
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

		cmd.SilenceUsage = true

		if err := state.View(cmd.Context(), w.Tracker.State, w.Tracker.Intent); err != nil {
			return fmt.Errorf("failed to view state: %w", err)
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(StateCmd)
	AddRefFlags(StateCmd, true)

	StateCmd.AddCommand(StateDiffCmd)
	StateCmd.AddCommand(StateGetCmd)
	StateCmd.AddCommand(StateMatchCmd)
	StateMatchCmd.Flags().BoolP("no-links", "l", false, "Do not match links.")

	StateCmd.AddCommand(StateSetIntentCmd)
	StateSetIntentCmd.Flags().StringP("format", "f", "string", "format of the input value. One of 'string', 'starlark' or 'json'.")

	StateApplyIntentCmd.Flags().Bool("cascade", false, "Cascade follow on work caused by updating this state")
	StateCmd.AddCommand(StateApplyIntentCmd)
	StateCmd.AddCommand(StateDeleteIntentCmd)

	StateCmd.AddCommand(StateViewCmd)
}
