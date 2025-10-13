package commands

import (
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

		worker, err := work.NewWorker(ctx, ref)
		if err != nil {
			return err
		}
		defer worker.Cleanup()

		return worker.Push(ctx)
	},
}

func init() {
	RootCmd.AddCommand(PushCmd)
}
