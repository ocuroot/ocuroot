package commands

import "github.com/spf13/cobra"

var StartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start long-running processes.",
	Long:  `Start long-running processes.`,
}

func init() {
	RootCmd.AddCommand(StartCmd)
}
