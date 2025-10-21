package commands

import (
	"fmt"
	"os"

	"github.com/ocuroot/ocuroot/client/preview"
	"github.com/ocuroot/ocuroot/client/work"
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
		ctx := cmd.Context()

		ref, err := GetRef(cmd, args)
		if err != nil {
			return fmt.Errorf("failed to get ref: %w", err)
		}

		w, err := work.NewInRepoWorker(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to create worker: %w", err)
		}
		w.Cleanup()

		if w.Tracker.Ref.HasRelease() {
			return fmt.Errorf("a release should not be specified for a preview")
		}

		// Clean the ref
		w.Tracker.Ref = w.Tracker.Ref.SetRelease("preview")

		packagePath := args[0]
		if _, err := os.Stat(packagePath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("package path does not exist: %w", err)
			}
			return fmt.Errorf("failed to stat package path: %w", err)
		}

		// Start web server
		preview.StartPreviewServer(w.Tracker, previewPort)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(PreviewCmd)

	// Add flags to the preview command
	PreviewCmd.Flags().IntVarP(&previewPort, "port", "p", 0, "Specify port for the preview server (uses an available port if not specified)")
}
