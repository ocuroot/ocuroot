package commands

import (
	"fmt"
	"strings"

	"github.com/ocuroot/ocuroot/about"
	"github.com/spf13/cobra"
)

var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Displays the current Ocuroot client version",
	Long:  `Displays the current Ocuroot client version.`,
	Run: func(cmd *cobra.Command, args []string) {
		versionParts := strings.Split(about.Version, "-")
		fmt.Println("Ocuroot version: " + versionParts[0])
		if len(versionParts) > 1 {
			fmt.Println("Build: " + versionParts[1])
		}
	},
}

func init() {
	RootCmd.AddCommand(VersionCmd)
}
