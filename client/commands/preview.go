package commands

import (
	"fmt"
	"os"

	"github.com/ocuroot/ocuroot/client/preview"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/spf13/cobra"
)

var (
	previewPort int
)

var PreviewCmd = &cobra.Command{
	Use:   "preview [package-file]",
	Short: "Preview package configuration",
	Long:  `Preview command allows you to visualize package configurations before deploying.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tc, err := getTrackerConfig(cmd, args)
		if err != nil {
			return err
		}

		if tc.Ref.ReleaseOrIntent.Type != refs.Unknown {
			return fmt.Errorf("a release should not be specified for a preview")
		}

		// Clean the ref
		tc.Ref = refs.Ref{
			Repo:     tc.Ref.Repo,
			Filename: tc.Ref.Filename,
			ReleaseOrIntent: refs.ReleaseOrIntent{
				Type:  refs.Release,
				Value: "preview",
			},
		}

		packagePath := args[0]
		if _, err := os.Stat(packagePath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("package path does not exist: %w", err)
			}
			return fmt.Errorf("failed to stat package path: %w", err)
		}

		// Start web server
		preview.StartPreviewServer(tc, previewPort)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(PreviewCmd)

	// Add flags to the preview command
	PreviewCmd.Flags().IntVarP(&previewPort, "port", "p", 0, "Specify port for the preview server (uses an available port if not specified)")
}
