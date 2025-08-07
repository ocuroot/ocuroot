package commands

import (
	"os"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/about"
	"github.com/spf13/cobra"
)

var cleanup func()

// rootCmd represents the base command for the client
// RootCmd represents the base command for the client
var RootCmd = &cobra.Command{
	Use:   "ocuroot",
	Short: "Ocuroot client CLI",
	Long: `Ocuroot client provides command-line tools for interacting 
with the Ocuroot release orchestration platform.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		log.Info("Starting ocuroot", "version", about.Version, "args", os.Args[1:])
		cleanup = setupTelemetry()
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		cleanup()
	},
}

// GetRootCommand returns the root Cobra command for the client
// GetRootCommand returns the root Cobra command for the client
func GetRootCommand() *cobra.Command {
	return RootCmd
}

// Execute runs the root command
func Execute() error {
	return RootCmd.Execute()
}
