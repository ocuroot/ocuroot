package commands

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/spf13/cobra"

	librelease "github.com/ocuroot/ocuroot/lib/release"
)

// DeployCmd represents the deploy command
var DeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Manage deployments",
	Long: `Deploy command allows you to manage deployments by performing 
operations like up (deploy) and down (undeploy).`,
}

// DeployUpCmd represents the deploy up command
var DeployUpCmd = &cobra.Command{
	Use:   "up [package-ref]",
	Short: "Start a deployment",
	Long: `Start a deployment by running the up operation.
	
This command creates and starts a new deployment for the specified package.

Example:
  ocuroot deploy up my-package-ref`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		packageRef := args[0]
		fmt.Printf("Running up operation for package: %s\n", packageRef)
		// TODO: Implement the actual up operation
		return nil
	},
}

// DeployDownCmd represents the deploy down command
var DeployDownCmd = &cobra.Command{
	Use:   "down [deployment-id]",
	Short: "Stop a deployment",
	Long: `Stop a deployment by running the down operation.
	
This command stops and removes the specified deployment.

Example:
  ocuroot deploy down my-deployment-id`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, span := tracer.Start(cmd.Context(), "ocuroot deploy down")
		defer span.End()

		tc, err := getTrackerConfig(cmd, args)
		if err != nil {
			return err
		}

		if tc.Ref.SubPathType != refs.SubPathTypeDeploy {
			return fmt.Errorf("deployment ID must be a deployment ref")
		}
		if strings.Contains(tc.Ref.SubPath, "/") {
			return fmt.Errorf("deployment ID must not contain a slash")
		}

		tracker, err := release.TrackerForExistingRelease(ctx, tc)
		if err != nil {
			return err
		}

		envName := tc.Ref.SubPath

		releaseStore, err := librelease.ReleaseStore(ctx, tc.Ref.String(), tc.Store)
		if err != nil {
			return err
		}

		err = releaseStore.InitDeployment(ctx, envName, true)
		if err != nil {
			return err
		}

		err = tracker.RunToPause(ctx, func(fnRef refs.Ref, l sdk.Log) {
			log.Info("log", "message", l.Message, "attributes", l.Attributes, "function", fnRef.String())
		})
		if err != nil {
			return err
		}

		return nil
	},
}

func init() {
	DeployCmd.AddCommand(DeployDownCmd)
	DeployCmd.AddCommand(DeployUpCmd)
	RootCmd.AddCommand(DeployCmd)
}
