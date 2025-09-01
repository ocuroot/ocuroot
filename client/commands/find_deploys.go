package commands

import (
	"fmt"
	"time"

	"github.com/ocuroot/gittools"
	librelease "github.com/ocuroot/ocuroot/lib/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/store/models"
	"github.com/spf13/cobra"
)

var FindDeploysCmd = &cobra.Command{
	Use:   "find-deploys [startCommit] [endCommit]",
	Short: "Find deployments containing specific commits",
	Long: `Find deployments that contain specific commits.
	
This command searches through the deployment history to locate any deployments
that include the changes between the specified start and end commits (inclusive).

Example:
  ocuroot find-deploys abc123 def456`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		tc, err := getTrackerConfig(ctx, cmd, args)
		if err != nil {
			return fmt.Errorf("failed to get tracker config: %w", err)
		}

		startCommit := args[0]
		endCommit := args[1]

		cmd.SilenceUsage = true

		// Get the repository path (assuming current directory if not specified)
		repoPath := tc.RepoPath

		// Open the repository
		repo, err := gittools.Open(repoPath)
		if err != nil {
			return fmt.Errorf("failed to open git repository: %w", err)
		}

		searchOpts := &gittools.GetCommitsBetweenOptions{
			MaxDepth:         1000,
			OperationTimeout: 30 * time.Second,
		}

		commits, err := repo.GetCommitsBetween(startCommit, endCommit, searchOpts)
		if err != nil {
			return fmt.Errorf("failed to search for commit %s: %w", startCommit, err)
		}

		refsToDeployment := make(map[string]models.Work)
		releaseToDeployment := make(map[string]librelease.ReleaseInfo)
		commitToRef := make(map[string]string)
		// Find all deploys from this repo
		reploymentRefs, err := tc.Store.Match(ctx, fmt.Sprintf("%v/-/**/@/deploy/*", tc.Ref.Repo))
		if err != nil {
			return fmt.Errorf("failed to match refs: %w", err)
		}
		for _, ref := range reploymentRefs {
			resolvedRef, err := tc.Store.ResolveLink(ctx, ref)
			if err != nil {
				return fmt.Errorf("failed to resolve ref: %w", err)
			}

			var work models.Work
			if err := tc.Store.Get(ctx, resolvedRef, &work); err != nil {
				return fmt.Errorf("failed to get work: %w", err)
			}
			refsToDeployment[ref] = work

			pr, err := refs.Parse(resolvedRef)
			if err != nil {
				return fmt.Errorf("failed to parse ref: %w", err)
			}
			releaseRef := pr.SetSubPathType(refs.SubPathTypeNone).SetSubPath("").SetFragment("")
			var releaseInfo librelease.ReleaseInfo
			if err := tc.Store.Get(ctx, releaseRef.String(), &releaseInfo); err != nil {
				return fmt.Errorf("failed to get release info (%v): %w", releaseRef.String(), err)
			}
			releaseToDeployment[releaseRef.String()] = releaseInfo

			commitToRef[releaseInfo.Commit] = ref
		}

		for _, commit := range commits {
			ref, exists := commitToRef[commit]
			if !exists {
				continue
			}
			fmt.Println(ref)
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(FindDeploysCmd)
}
