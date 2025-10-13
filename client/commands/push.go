package commands

import (
	"encoding/json"
	"fmt"

	"github.com/ocuroot/ocuroot/client/work"
	"github.com/spf13/cobra"
)

var PushCmd = &cobra.Command{
	Use:   "push",
	Short: "Handle a push to a git repo",
	Long:  `Handle a push to a git repo`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		ref, err := GetRef(nil, nil)
		if err != nil {
			return err
		}

		dryRun, err := cmd.Flags().GetBool("dryrun")
		if err != nil {
			return err
		}

		worker, err := work.NewWorker(ctx, ref)
		if err != nil {
			return err
		}
		defer worker.Cleanup()

		if dryRun {
			work, err := worker.PushWork(ctx)
			if err != nil {
				return err
			}

			workJSON, err := json.MarshalIndent(work, "", "  ")
			if err != nil {
				return err
			}
			worker.Cleanup()
			fmt.Println(string(workJSON))

			return nil
		}

		return worker.Push(ctx)
	},
}

func init() {

	PushCmd.Flags().BoolP("dryrun", "n", false, "List releases that would be executed")
	RootCmd.AddCommand(PushCmd)
}
