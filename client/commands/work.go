package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"reflect"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/local"
	"github.com/ocuroot/ocuroot/client/release"
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

		logMode := cmd.Flag("logmode").Changed

		tc, err := getTrackerConfig(cmd, args)
		if err != nil {
			return fmt.Errorf("failed to get tracker config: %w", err)
		}

		// Update inputs for existing deployments
		if err := reconcileAllDeploymentsAtCommit(ctx, tc.Store, tc.Ref.Repo, tc.Commit); err != nil {
			return fmt.Errorf("failed to reconcile deployments: %w", err)
		}

		// Match any outstanding functions for this repo/commit
		mr := fmt.Sprintf(
			"%v/-/**/@%v*/{deploy,call}/**/functions/*/status/pending",
			tc.Ref.Repo,
			tc.Commit,
		)
		outstanding, err := tc.Store.Match(
			cmd.Context(),
			mr)
		if err != nil {
			return fmt.Errorf("failed to match refs: %w", err)
		}

		releases := make(map[refs.Ref]struct{})
		for _, ref := range outstanding {
			log.Info("Found outstanding function", "ref", ref)
			pr, err := refs.Parse(ref)
			if err != nil {
				return fmt.Errorf("failed to parse ref: %w", err)
			}
			pr = pr.SetSubPathType(refs.SubPathTypeNone)
			pr = pr.SetSubPath("")
			pr = pr.SetFragment("")
			releases[pr] = struct{}{}
		}

		if len(releases) == 0 {
			fmt.Println("No work to continue")
			return nil
		}

		for releaseRef := range releases {
			fmt.Println("Continuing release: " + releaseRef.String())
			tc.Ref = releaseRef

			if err := continueRelease(ctx, tc, logMode); err != nil {
				return err
			}
		}

		return nil
	},
}

var WorkTriggerCommand = &cobra.Command{
	Use:   "trigger",
	Short: "Trigger outstanding work in the state store",
	Long:  `Trigger outstanding work in the state store.`,
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		store, err := getReadOnlyStore()
		if err != nil {
			return fmt.Errorf("failed to get read only store: %w", err)
		}

		dryRun := cmd.Flag("dryrun").Changed
		intent := cmd.Flag("intent").Changed
		if intent {
			// Match any outstanding functions in the state repo
			mr := "**/-/repo.ocu.star/+"
			repo, err := store.Match(
				ctx,
				mr)
			if err != nil {
				fmt.Println("Failed to match refs: " + err.Error())
				return nil
			}

			for _, ref := range repo {
				resolved, err := store.ResolveLink(ctx, ref)
				if err != nil {
					fmt.Println("Failed to resolve link: " + err.Error())
					continue
				}

				if err := triggerWork(ctx, store, resolved, dryRun); err != nil {
					fmt.Println("Failed to trigger work against " + resolved + ": " + err.Error())
				}
			}

			return nil
		}

		// Match any outstanding functions in the state repo
		mr := "**/@*/{deploy,call}/**/functions/*/status/pending"
		outstanding, err := store.Match(
			ctx,
			mr)
		if err != nil {
			return fmt.Errorf("failed to match refs: %w", err)
		}

		reconcilable, err := getReconcilableDeployments(ctx, store)
		if err != nil {
			return fmt.Errorf("failed to get reconcilable deployments: %w", err)
		}

		type RepoCommitTuple struct {
			Repo   string
			Commit string
		}

		repos := make(map[RepoCommitTuple]struct{})
		for _, ref := range outstanding {
			funcReady, err := librelease.FunctionIsReady(ctx, store, ref)
			if err != nil {
				return fmt.Errorf("failed to check function: %w", err)
			}
			if !funcReady {
				continue
			}

			if dryRun {
				fmt.Println("Outstanding ref: " + ref)
			}
			pr, err := refs.Parse(ref)
			if err != nil {
				return fmt.Errorf("failed to parse ref: %w", err)
			}
			repos[RepoCommitTuple{
				Repo:   pr.Repo,
				Commit: strings.Split(pr.ReleaseOrIntent.Value, ".")[0],
			}] = struct{}{}
		}

		for _, ref := range reconcilable {
			if dryRun {
				fmt.Println("Reconcilable ref: " + ref.String())
			}
			repos[RepoCommitTuple{
				Repo:   ref.Repo,
				Commit: strings.Split(ref.ReleaseOrIntent.Value, ".")[0],
			}] = struct{}{}
		}

		for repoCommit := range repos {
			configRef := repoCommit.Repo + "/-/repo.ocu.star/@" + repoCommit.Commit
			if err := triggerWork(ctx, store, configRef, dryRun); err != nil {
				fmt.Println("Failed to trigger work: " + err.Error())
			}
		}

		return nil
	},
}

var WorkTasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Run scheduled tasks",
	Long:  `Run scheduled tasks against this commit.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		tc, err := getTrackerConfig(cmd, args)
		if err != nil {
			return fmt.Errorf("failed to get tracker config: %w", err)
		}

		// Match any outstanding tasks for this repo
		mr := fmt.Sprintf(
			"%v/-/**/@%v*/task/*",
			tc.Ref.Repo,
			tc.Commit,
		)
		log.Info("Checking for tasks", "glob", mr)
		tasks, err := tc.Store.Match(
			ctx,
			mr)
		if err != nil {
			return fmt.Errorf("failed to match refs: %w", err)
		}

		for _, ref := range tasks {
			log.Info("Found outstanding task", "ref", ref)
			if err := runTask(ctx, tc, ref); err != nil {
				return fmt.Errorf("failed to run task: %w", err)
			}
		}

		return nil
	},
}

func runTask(ctx context.Context, tc release.TrackerConfig, ref string) error {
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
			fmt.Println("The specified release was not found. " + tc.Ref.String())
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

func continueRelease(ctx context.Context, tc release.TrackerConfig, logMode bool) error {
	workTui := tui.StartWorkTui(logMode)
	defer workTui.Cleanup()

	tc.Store = tuiwork.WatchForChainUpdates(tc.Store, workTui)

	tracker, err := release.TrackerForExistingRelease(ctx, tc)
	if err != nil {
		if errors.Is(err, refstore.ErrRefNotFound) {
			fmt.Println("The specified release was not found. " + tc.Ref.String())
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
		fmt.Println("Repo:", configRef)
		return nil
	}

	fmt.Println("Triggering work for repo: " + configRef)
	var repoConfig RepoConfig
	if err := readOnlyStore.Get(ctx, configRef, &repoConfig); err != nil {
		return fmt.Errorf("failed to get repo config: %w", err)
	}

	backend, be := local.BackendForRepo()

	_, err := sdk.LoadRepoFromBytes(
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
		thread := &starlark.Thread{
			Name: "repo-trigger",
			Print: func(thread *starlark.Thread, msg string) {
				fmt.Println(msg)
			},
		}
		pr, err := refs.Parse(configRef)
		if err != nil {
			return fmt.Errorf("failed to parse ref: %w", err)
		}
		commit := pr.ReleaseOrIntent.Value

		_, err = starlark.Call(thread, be.RepoTrigger, starlark.Tuple{starlark.String(commit)}, nil)
		if err != nil {
			return fmt.Errorf("failed to call repo trigger: %w", err)
		}
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
	var deployment models.Work
	if err := store.Get(ctx, ref, &deployment); err != nil {
		return nil, fmt.Errorf("failed to get deployment at %s: %w", ref, err)
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

	entryFunction := deployment.Entrypoint
	var entryFunctionState librelease.FunctionState
	if err := store.Get(ctx, entryFunction.String(), &entryFunctionState); err != nil {
		return nil, fmt.Errorf("failed to get entry function at %s: %w", entryFunction.String(), err)
	}

	dependenciesSatisfied, err := librelease.CheckDependencies(ctx, store, entryFunctionState)
	if err != nil {
		return nil, fmt.Errorf("failed to check dependencies: %w", err)
	}
	// Don't attempt to trigger if the dependencies haven't been satisfied
	if !dependenciesSatisfied {
		return nil, nil
	}

	entryFunctionInputs := entryFunctionState.Current.Inputs

	inputs, err := librelease.PopulateInputs(ctx, store, entryFunctionInputs)
	if err != nil {
		return nil, fmt.Errorf("failed to populate inputs: %w", err)
	}

	var changed bool
	for k, v := range inputs {
		// Ensure we don't create loops with the outputs of this chain
		if isForSameWork(*v.Ref, parsedResolvedDeployment) {
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

func isForSameWork(ref1, ref2 refs.Ref) bool {
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
func reconcileAllDeploymentsAtCommit(ctx context.Context, store refstore.Store, repo, commit string) error {
	log.Info("Reconciling all deployments")
	allDeployments, err := store.Match(ctx, fmt.Sprintf("%v/-/**/@/deploy/*", repo))
	if err != nil {
		return fmt.Errorf("failed to match deployments: %w", err)
	}

	for _, ref := range allDeployments {
		var deployment models.Work
		if err := store.Get(ctx, ref, &deployment); err != nil {
			log.Error("Failed to get deployment", "ref", ref, "error", err)
			continue
		}

		resolvedDeployment, err := store.ResolveLink(ctx, ref)
		if err != nil {
			log.Error("Failed to resolve inputs", "ref", ref, "error", err)
			continue
		}
		parsedResolvedDeployment, err := refs.Parse(resolvedDeployment)
		if err != nil {
			log.Error("Failed to parse resolved deployment", "ref", ref, "error", err)
			continue
		}

		var release librelease.ReleaseInfo
		if err := store.Get(ctx, parsedResolvedDeployment.SetSubPathType(refs.SubPathTypeNone).SetSubPath("").SetFragment("").String(), &release); err != nil {
			log.Error("Failed to get release", "ref", ref, "error", err)
			continue
		}

		if release.Commit != commit {
			log.Info("Deployment is not for the correct commit", "ref", ref, "commit", commit)
			continue
		}

		entryFunction := deployment.Entrypoint
		var entryFunctionState librelease.FunctionState
		if err := store.Get(ctx, entryFunction.String(), &entryFunctionState); err != nil {
			log.Error("Failed to get entry function", "ref", ref, "error", err)
			continue
		}
		entryFunctionInputs := entryFunctionState.Current.Inputs

		inputs, err := librelease.PopulateInputs(ctx, store, entryFunctionInputs)
		if err != nil {
			continue
		}

		var changed bool
		for k, v := range inputs {
			if !reflect.DeepEqual(entryFunctionInputs[k].Value, v.Value) {
				// Ensure we don't create loops with the outputs of this chain
				if isForSameWork(*v.Ref, parsedResolvedDeployment) {
					continue
				}

				log.Info("input changed", "key", k, "oldValue", toJSON(entryFunctionInputs[k].Value), "newValue", v.Value, "vRef", v.Ref.String(), "parsedResolvedDeployment", parsedResolvedDeployment.String())
				changed = true
			}
		}

		// Nothing more to be done if the inputs haven't changed
		if !changed {
			continue
		}

		log.Info("Duplicating deployment", "ref", resolvedDeployment)
		parsedResolvedDeployment, err = refs.Parse(resolvedDeployment)
		if err != nil {
			log.Error("Failed to parse resolved deployment", "ref", ref, "error", err)
			continue
		}
		newFunctionChainRef := parsedResolvedDeployment.SetSubPath(path.Join(path.Dir(parsedResolvedDeployment.SubPath)))
		newFunctionChainRefString, err := refstore.IncrementPath(ctx, store, fmt.Sprintf("%s/", newFunctionChainRef.String()))
		if err != nil {
			log.Error("Failed to increment path", "ref", ref, "error", err)
			continue
		}
		newFunctionChainRef, err = refs.Parse(newFunctionChainRefString)
		if err != nil {
			log.Error("Failed to parse path", "ref", ref, "error", err)
			continue
		}
		err = librelease.InitializeFunctionChain(ctx, store, newFunctionChainRef, &models.Function{
			ID:     "1",
			Fn:     entryFunctionState.Current.Fn,
			Status: models.StatusPending,
			Inputs: inputs,
		})
		if err != nil {
			log.Error("Failed to initialize function chain", "ref", ref, "error", err)
			continue
		}

		log.Info("Duplicated deployment", "oldRef", resolvedDeployment, "newRef", newFunctionChainRef)
	}

	return nil
}

func init() {
	RootCmd.AddCommand(WorkCmd)

	WorkContinueCmd.Flags().BoolP("logmode", "l", false, "Enable log mode when initializing the TUI")
	WorkCmd.AddCommand(WorkContinueCmd)

	WorkTriggerCommand.Flags().BoolP("dryrun", "d", false, "List refs for work that would be triggered")
	WorkTriggerCommand.Flags().BoolP("intent", "i", false, "Trigger intents instead of deployments")
	WorkCmd.AddCommand(WorkTriggerCommand)

	WorkCmd.AddCommand(WorkTasksCmd)
}
