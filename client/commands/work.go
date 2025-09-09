package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/local"
	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/client/state"
	"github.com/ocuroot/ocuroot/client/tui"
	"github.com/ocuroot/ocuroot/client/tui/tuiwork"
	librelease "github.com/ocuroot/ocuroot/lib/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"
	"github.com/spf13/cobra"
	"go.starlark.net/starlark"
)

var WorkCmd = &cobra.Command{
	Use:   "work",
	Short: "Manage outstanding work",
	Long:  `Manage outstanding work.`,
}

var WorkContinueCmd = &cobra.Command{
	Use:   "continue",
	Short: "Continue outstanding work",
	Long:  `Continue outstanding work against the current commit.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		tc, err := getTrackerConfig(ctx, cmd, args)
		if err != nil {
			return fmt.Errorf("failed to get tracker config: %w", err)
		}

		if err := doRunsForCommit(ctx, tc); err != nil {
			return err
		}

		return nil
	},
}

var WorkAnyCommand = &cobra.Command{
	Use:   "any",
	Short: "Pick up any outstanding work",
	Long: `Pick up any outstanding work based on the contents of the state store.

Will start by running any release work (equivalent to 'ocuroot work continue'), then
any ops ('ocuroot work ops'), then sync any intent ('ocuroot state diff | xargs -r -n1 ocuroot state apply'),
finally it will trigger work for other commits ('ocuroot work trigger').
	`,
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		tc, err := getTrackerConfig(ctx, cmd, args)
		if err != nil {
			return fmt.Errorf("failed to get tracker config: %w", err)
		}

		cmd.SilenceUsage = true

		log.Info("Starting state sync")
		if err := state.Sync(ctx, tc.State, tc.Intent); err != nil {
			return err
		}

		log.Info("Starting op work")
		if err := doOps(ctx, tc); err != nil {
			return err
		}

		log.Info("Starting release work")
		if err := doRunsForCommit(ctx, tc); err != nil {
			return err
		}

		log.Info("Starting trigger work")
		if err := doTriggerWork(ctx, tc.State, tc.Intent, false); err != nil {
			return err
		}

		return nil
	},
}

func doRunsForCommit(ctx context.Context, tc release.TrackerConfig) error {
	// Update inputs for existing deployments
	if err := reconcileAllDeploymentsAtCommit(ctx, tc.State, tc.Ref.Repo, tc.Commit); err != nil {
		return fmt.Errorf("failed to reconcile deployments: %w", err)
	}

	// Match any outstanding work for this repo/commit
	mrPending := fmt.Sprintf(
		"%v/-/**/@%v*/{deploy,task}/*/*/status/pending",
		tc.Ref.Repo,
		tc.Commit,
	)
	mrPaused := fmt.Sprintf(
		"%v/-/**/@%v*/{deploy,task}/*/*/status/paused",
		tc.Ref.Repo,
		tc.Commit,
	)
	outstanding, err := tc.State.Match(
		ctx,
		mrPending,
		mrPaused,
	)
	if err != nil {
		return fmt.Errorf("failed to match refs: %w", err)
	}

	releasesForCommit, err := releasesForCommit(ctx, tc.State, tc.Ref.Repo, tc.Commit)
	if err != nil {
		return fmt.Errorf("failed to get releases for commit: %w", err)
	}
	for _, ref := range releasesForCommit {
		log.Info("Found release for commit", "ref", ref)
		outstandingRuns, err := tc.State.Match(
			ctx,
			fmt.Sprintf("%v/{deploy,task}/*/*/status/pending", ref.String()),
			fmt.Sprintf("%v/{deploy,task}/*/*/status/paused", ref.String()))
		if err != nil {
			return fmt.Errorf("failed to match refs: %w", err)
		}
		outstanding = append(outstanding, outstandingRuns...)
	}

	releases := make(map[refs.Ref]struct{})
	for _, ref := range outstanding {
		log.Info("Found outstanding run", "ref", ref)
		pr, err := refs.Parse(ref)
		if err != nil {
			return fmt.Errorf("failed to parse ref: %w", err)
		}
		pr = pr.SetSubPathType(refs.SubPathTypeNone).
			SetSubPath("").
			SetFragment("")
		releases[pr] = struct{}{}
	}

	if len(releases) == 0 {
		fmt.Println("No runs to continue")
		return nil
	}

	workTui := tui.StartWorkTui()
	defer workTui.Cleanup()

	tc.State = tuiwork.WatchForJobUpdates(ctx, tc.State, workTui)

	for releaseRef := range releases {
		tc.Ref = releaseRef

		if err := continueRelease(ctx, tc, workTui); err != nil {
			return err
		}
	}

	return nil
}

var WorkTriggerCommand = &cobra.Command{
	Use:   "trigger",
	Short: "Trigger outstanding runs in the state store",
	Long:  `Trigger outstanding runs in the state store.`,
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		store, err := getReadOnlyStore(ctx)
		if err != nil {
			return fmt.Errorf("failed to get read only store: %w", err)
		}

		dryRun := cmd.Flag("dryrun").Changed
		intent := cmd.Flag("intent").Changed
		if intent {
			log.Info("Triggering in intent mode")
			// Match repo config files to identify unique repos
			mr := "**/-/repo.ocu.star/{+,@}"
			repo, err := store.Match(
				ctx,
				mr)
			if err != nil {
				fmt.Println("Failed to match refs: " + err.Error())
				return nil
			}

			log.Info("Repo matches", "count", len(repo), "repo", repo)

			for _, ref := range repo {
				if err := triggerWork(ctx, store, ref, dryRun); err != nil {
					fmt.Println("Failed to trigger run against " + ref + ": " + err.Error())
				}
			}

			return nil
		}

		return doTriggerWork(ctx, store, store, dryRun)
	},
}

func doTriggerWork(ctx context.Context, state, intent refstore.Store, dryRun bool) error {
	repos := make(map[RepoCommitTuple]struct{})

	// Match any outstanding runs in the state repo
	outstanding, err := state.Match(
		ctx,
		"**/@*/{deploy,task}/*/*/status/pending",
		"**/@*/{deploy,task}/*/*/status/paused",
	)
	if err != nil {
		return fmt.Errorf("failed to match refs: %w", err)
	}

	for _, ref := range outstanding {
		funcReady, err := librelease.RunIsReady(ctx, state, ref)
		if err != nil {
			return fmt.Errorf("failed to check run: %w", err)
		}
		if !funcReady {
			continue
		}

		if dryRun {
			fmt.Println("Outstanding ref: " + ref)
		}

		repoCommit, err := getRepoAndCommitForRelease(ctx, ref, state)
		if err != nil {
			return fmt.Errorf("failed to get repo and commit for release: %w", err)
		}
		repos[repoCommit] = struct{}{}
	}

	// Identify any reconcilable deployments (where inputs have changed)
	reconcilable, err := getReconcilableDeployments(ctx, state)
	if err != nil {
		return fmt.Errorf("failed to get reconcilable deployments: %w", err)
	}

	for _, ref := range reconcilable {
		if dryRun {
			fmt.Println("Reconcilable ref: " + ref.String())
		}
		repoCommit, err := getRepoAndCommitForRelease(ctx, ref.String(), state)
		if err != nil {
			return fmt.Errorf("failed to get repo and commit for release: %w", err)
		}
		repos[repoCommit] = struct{}{}
	}

	// Identify any ops
	outstanding, err = state.Match(
		ctx,
		"**/@*/op/*",
	)
	if err != nil {
		return fmt.Errorf("failed to match refs: %w", err)
	}

	for _, ref := range outstanding {
		log.Info("Found outstanding op", "ref", ref)
		if dryRun {
			fmt.Println("Outstanding ref: " + ref)
		}
		repoCommit, err := getRepoAndCommitForRelease(ctx, ref, state)
		if err != nil {
			return fmt.Errorf("failed to get repo and commit for release: %w", err)
		}
		repos[repoCommit] = struct{}{}
	}

	for repoCommit := range repos {
		configRef := repoCommit.Repo + "/-/repo.ocu.star/@" + repoCommit.Commit
		if err := triggerWork(ctx, state, configRef, dryRun); err != nil {
			fmt.Println("Failed to trigger run: " + err.Error())
		}
	}

	return nil
}

var WorkOpsCmd = &cobra.Command{
	Use:   "ops",
	Short: "Run scheduled ops",
	Long:  `Run scheduled operations against this commit.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		tc, err := getTrackerConfig(ctx, cmd, args)
		if err != nil {
			return fmt.Errorf("failed to get tracker config: %w", err)
		}

		if err := doOps(ctx, tc); err != nil {
			return err
		}

		return nil
	},
}

func doOps(ctx context.Context, tc release.TrackerConfig) error {
	releasesForCommit, err := releasesForCommit(ctx, tc.State, tc.Ref.Repo, tc.Commit)
	if err != nil {
		return fmt.Errorf("failed to get releases for commit: %w", err)
	}

	var ops []string
	for _, ref := range releasesForCommit {
		mr := fmt.Sprintf("%v/op/*", ref.String())
		log.Info("Checking for ops", "glob", mr)
		matchedOps, err := tc.State.Match(ctx, mr)
		if err != nil {
			return fmt.Errorf("failed to match refs: %w", err)
		}
		ops = append(ops, matchedOps...)
	}

	for _, ref := range ops {
		log.Info("Found outstanding op", "ref", ref)
		if err := runOp(ctx, tc, ref); err != nil {
			return fmt.Errorf("failed to run op: %w", err)
		}
	}
	return nil
}

func runOp(ctx context.Context, tc release.TrackerConfig, ref string) error {
	var err error

	tc.Ref, err = refs.Parse(ref)
	if err != nil {
		return fmt.Errorf("failed to parse ref: %w", err)
	}
	tc.Ref.SubPath = ""
	tc.Ref.SubPathType = refs.SubPathTypeNone

	log.Info("Setting up tracker", "tc", tc)
	tracker, err := release.TrackerForExistingRelease(ctx, tc)
	if err != nil {
		if errors.Is(err, refstore.ErrRefNotFound) {
			log.Error("The specified release was not found", "ref", tc.Ref.String())
			return nil
		}
		return fmt.Errorf("failed to get tracker: %w", err)
	}

	err = tracker.Task(ctx, ref, nil)
	if err != nil {
		return fmt.Errorf("running task in tracker: %w", err)
	}

	return nil
}

func continueRelease(ctx context.Context, tc release.TrackerConfig, workTui tui.Tui) error {
	tracker, err := release.TrackerForExistingRelease(ctx, tc)
	if err != nil {
		if errors.Is(err, refstore.ErrRefNotFound) {
			log.Error("The specified release was not found", "ref", tc.Ref.String())
			return nil
		}
		return err
	}

	err = tracker.RunToPause(
		ctx,
		tuiwork.TuiLogger(workTui),
	)
	if err != nil {
		return err
	}

	return nil
}

func triggerWork(ctx context.Context, readOnlyStore refstore.Store, configRef string, dryRun bool) error {
	if dryRun {
		return nil
	}

	configWithCommit, err := readOnlyStore.ResolveLink(ctx, configRef)
	if err != nil {
		return fmt.Errorf("failed to resolve config ref (%v): %w", configRef, err)
	}

	log.Info("Triggering work for repo", "ref", configWithCommit)
	var repoConfig models.RepoConfig
	if err := readOnlyStore.Get(ctx, configWithCommit, &repoConfig); err != nil {
		return fmt.Errorf("failed to get repo config (%v): %w", configWithCommit, err)
	}

	backend, be := local.BackendForRepo()

	_, err = sdk.LoadRepoFromBytes(
		ctx,
		sdk.NewNullResolver(),
		"repo.ocu.star",
		repoConfig.Source,
		backend,
		func(thread *starlark.Thread, msg string) {},
	)
	if err != nil {
		return fmt.Errorf("failed to load repo: %w", err)
	}

	if be.RepoTrigger != nil {
		log.Info("Executing repo trigger", "ref", configWithCommit)

		thread := &starlark.Thread{
			Name: "repo-trigger",
			Print: func(thread *starlark.Thread, msg string) {
				log.Info("Repo trigger", "msg", msg)
				fmt.Println(msg)
			},
		}
		pr, err := refs.Parse(configWithCommit)
		if err != nil {
			return fmt.Errorf("failed to parse ref: %w", err)
		}
		commit := pr.Release

		_, err = starlark.Call(thread, be.RepoTrigger, starlark.Tuple{starlark.String(commit)}, nil)
		if err != nil {
			return fmt.Errorf("failed to call repo trigger: %w", err)
		}
	} else {
		log.Info("No repo trigger found")
	}

	return nil
}

func toJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}

func getReconcilableDeployments(ctx context.Context, store refstore.Store) ([]refs.Ref, error) {
	var out []refs.Ref

	log.Info("Getting reconcilable deployments")
	allDeployments, err := store.Match(ctx, "**/@/deploy/*")
	if err != nil {
		return nil, fmt.Errorf("failed to match deployments: %w", err)
	}

	for _, ref := range allDeployments {
		resolved, err := reconcileDeployment(ctx, store, ref)
		if err != nil {
			log.Error("Failed to reconcile deployment", "ref", ref, "error", err)
			continue
		}
		if resolved != nil {
			out = append(out, *resolved)
		}
	}

	return out, nil
}

func reconcileDeployment(ctx context.Context, store refstore.Store, ref string) (*refs.Ref, error) {
	var deployment models.Task
	if err := store.Get(ctx, ref, &deployment); err != nil {
		return nil, fmt.Errorf("failed to get deployment at %s: %w", ref, err)
	}
	var run models.Run
	if err := store.Get(ctx, deployment.RunRef.String(), &run); err != nil {
		return nil, fmt.Errorf("failed to get run at %s: %w", deployment.RunRef.String(), err)
	}

	resolvedDeployment, err := store.ResolveLink(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve inputs at %s: %w", ref, err)
	}
	parsedResolvedDeployment, err := refs.Parse(resolvedDeployment)
	if err != nil {
		return nil, fmt.Errorf("failed to parse resolved deployment at %s: %w", ref, err)
	}

	var release librelease.ReleaseInfo
	if err := store.Get(ctx, parsedResolvedDeployment.SetSubPathType(refs.SubPathTypeNone).SetSubPath("").SetFragment("").String(), &release); err != nil {
		return nil, fmt.Errorf("failed to get release: %w", err)
	}

	entryFunction := run.Functions[0]
	dependenciesSatisfied, err := librelease.CheckDependencies(ctx, store, entryFunction)
	if err != nil {
		return nil, fmt.Errorf("failed to check dependencies: %w", err)
	}
	// Don't attempt to trigger if the dependencies haven't been satisfied
	if !dependenciesSatisfied {
		return nil, nil
	}

	entryFunctionInputs := entryFunction.Inputs
	inputs, err := librelease.PopulateInputs(ctx, store, entryFunctionInputs)
	if err != nil {
		return nil, fmt.Errorf("failed to populate inputs: %w", err)
	}

	var changed bool
	for k, v := range inputs {
		// Ensure we don't create loops with the outputs of this job
		if v.Ref != nil && isForSameJob(*v.Ref, parsedResolvedDeployment) {
			continue
		}

		if !reflect.DeepEqual(entryFunctionInputs[k].Value, v.Value) {
			log.Info("input changed", "key", k, "oldValue", toJSON(entryFunctionInputs[k].Value), "newValue", v.Value, "vRef", v.Ref.String(), "parsedResolvedDeployment", parsedResolvedDeployment.String())
			changed = true
		}
	}

	// Nothing more to be done if the inputs haven't changed
	if !changed {
		return nil, nil
	}

	return &parsedResolvedDeployment, nil
}

func isForSameJob(ref1, ref2 refs.Ref) bool {
	sub1 := ref1.SubPath
	sub1 = strings.SplitN(sub1, "/", 2)[0]

	sub2 := ref2.SubPath
	sub2 = strings.SplitN(sub2, "/", 2)[0]

	return ref1.Repo == ref2.Repo &&
		ref1.Filename == ref2.Filename &&
		sub1 == sub2 &&
		ref1.SubPathType == ref2.SubPathType
}

// reconcileAllDeployments attempts to resolve the inputs for all deployments
// Any inputs that changed will result in a redeployment with the updated value
func reconcileAllDeploymentsAtCommit(ctx context.Context, state refstore.Store, repo, commit string) error {
	log.Info("Reconciling all deployments")
	allDeployments, err := state.Match(ctx, fmt.Sprintf("%v/-/**/@/deploy/*", repo))
	if err != nil {
		return fmt.Errorf("failed to match deployments: %w", err)
	}

	for _, ref := range allDeployments {
		var deployment models.Task
		if err := state.Get(ctx, ref, &deployment); err != nil {
			log.Error("Failed to get deployment", "ref", ref, "error", err)
			continue
		}
		var run models.Run
		if err := state.Get(ctx, deployment.RunRef.String(), &run); err != nil {
			log.Error("Failed to get run", "ref", deployment.RunRef.String(), "error", err)
			continue
		}

		resolvedDeployment, err := state.ResolveLink(ctx, ref)
		if err != nil {
			log.Error("Failed to resolve inputs", "ref", ref, "error", err)
			continue
		}
		parsedResolvedTask, err := refs.Parse(resolvedDeployment)
		if err != nil {
			log.Error("Failed to parse resolved deployment", "ref", ref, "error", err)
			continue
		}

		var release librelease.ReleaseInfo
		if err := state.Get(ctx, parsedResolvedTask.SetSubPathType(refs.SubPathTypeNone).SetSubPath("").SetFragment("").String(), &release); err != nil {
			log.Error("Failed to get release", "ref", ref, "error", err)
			continue
		}

		if release.Commit != commit {
			log.Info("Deployment is not for the correct commit", "ref", ref, "commit", commit)
			continue
		}

		entryFunctionInputs := deployment.Inputs
		inputs, err := librelease.PopulateInputs(ctx, state, entryFunctionInputs)
		if err != nil {
			continue
		}

		var changed bool
		for k, v := range inputs {
			if !reflect.DeepEqual(entryFunctionInputs[k].Value, v.Value) {
				// Ensure we don't create loops with the outputs of this job
				if isForSameJob(*v.Ref, parsedResolvedTask) {
					continue
				}

				log.Info("input changed", "key", k, "oldValue", toJSON(entryFunctionInputs[k].Value), "newValue", v.Value, "vRef", v.Ref.String(), "parsedResolvedDeployment", parsedResolvedTask.String())
				changed = true
			}
		}

		// Nothing more to be done if the inputs haven't changed
		if !changed {
			continue
		}

		log.Info("Duplicating deployment", "ref", resolvedDeployment)
		parsedResolvedTask, err = refs.Parse(resolvedDeployment)
		if err != nil {
			log.Error("Failed to parse resolved deployment", "ref", ref, "error", err)
			continue
		}
		newRunRefString, err := refstore.IncrementPath(ctx, state, fmt.Sprintf("%s/", parsedResolvedTask.String()))
		if err != nil {
			log.Error("Failed to increment path", "ref", ref, "error", err)
			continue
		}
		log.Info("Incremented path", "ref", ref, "newRef", newRunRefString)
		newRunRef, err := refs.Parse(newRunRefString)
		if err != nil {
			log.Error("Failed to parse path", "ref", ref, "error", err)
			continue
		}
		err = librelease.InitializeRun(ctx, state, newRunRef, &models.Function{
			Fn:     run.Functions[0].Fn,
			Inputs: inputs,
		})
		if err != nil {
			log.Error("Failed to initialize run", "ref", ref, "error", err)
			continue
		}

		log.Info("Duplicated deployment", "oldRef", resolvedDeployment, "newRef", newRunRef)
	}

	return nil
}

func init() {
	RootCmd.AddCommand(WorkCmd)

	WorkCmd.AddCommand(WorkContinueCmd)

	WorkTriggerCommand.Flags().BoolP("dryrun", "d", false, "List refs for work that would be triggered")
	WorkTriggerCommand.Flags().BoolP("intent", "i", false, "Trigger intents instead of deployments")
	WorkCmd.AddCommand(WorkTriggerCommand)

	WorkCmd.AddCommand(WorkOpsCmd)
	WorkCmd.AddCommand(WorkAnyCommand)
}
