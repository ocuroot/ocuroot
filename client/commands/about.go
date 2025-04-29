package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var AboutCmd = &cobra.Command{
	Use:   "about",
	Short: "Display information about Ocuroot",
	Long:  `Provides detailed information about the Ocuroot release orchestration tool.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Ocuroot: Release Orchestration Tool")
		fmt.Println("Version: 0.1.0")
		fmt.Println("A tool for managing releases in mid-sized organizations.")
	},
}

func init() {
	RootCmd.AddCommand(AboutCmd)
}
