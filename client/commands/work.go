package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/local"
	"github.com/ocuroot/ocuroot/client/tui"
	"github.com/ocuroot/ocuroot/client/tui/tuiwork"
	"github.com/ocuroot/ocuroot/client/work"
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

		cmd.SilenceUsage = true
		dryRun := cmd.Flag("dryrun").Changed

		workTui := tui.StartWorkTui()
		defer workTui.Cleanup()

		tc.State = tuiwork.WatchForJobUpdates(ctx, tc.State, workTui)

		worker := &work.Worker{
			Tracker: tc,
			Tui:     workTui,
		}

		todo, err := worker.IdentifyWork(ctx, work.IndentifyWorkRequest{
			GitFilter: work.GitFilterCurrentCommitOnly,
		})
		if err != nil {
			return fmt.Errorf("failed to identify work: %w", err)
		}

		log.Info("Identified work", "todo", toJSON(todo))

		if dryRun {
			workTui.Cleanup()

			todoJSON, err := json.MarshalIndent(todo, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal todo: %w", err)
			}
			fmt.Println(string(todoJSON))
			return nil
		}

		if err := worker.ExecuteWork(ctx, todo); err != nil {
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
		dryRun := cmd.Flag("dryrun").Changed

		workTui := tui.StartWorkTui()
		defer workTui.Cleanup()

		tc.State = tuiwork.WatchForJobUpdates(ctx, tc.State, workTui)

		worker := &work.Worker{
			Tracker: tc,
			Tui:     workTui,
		}

		todo, err := worker.IdentifyWork(ctx, work.IndentifyWorkRequest{
			GitFilter: work.GitFilterCurrentCommitOnly,
		})
		if err != nil {
			return fmt.Errorf("failed to identify work: %w", err)
		}

		log.Info("Identified work", "todo", toJSON(todo))

		if dryRun {
			workTui.Cleanup()

			todoJSON, err := json.MarshalIndent(todo, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal todo: %w", err)
			}
			fmt.Println(string(todoJSON))
			return nil
		}

		if err := worker.ExecuteWork(ctx, todo); err != nil {
			return err
		}

		log.Info("Starting trigger work")
		if err := doTriggerWork(ctx, tc.State, tc.Intent, false); err != nil {
			return err
		}

		return nil
	},
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

		dryRun := cmd.Flag("dryrun").Changed

		worker := &work.Worker{
			Tracker: tc,
		}

		todo, err := worker.Ops(ctx, work.IndentifyWorkRequest{
			GitFilter: work.GitFilterCurrentCommitOnly,
		})
		if err != nil {
			return fmt.Errorf("failed to identify work: %w", err)
		}

		log.Info("Identified work", "todo", toJSON(todo))

		if dryRun {
			todoJSON, err := json.MarshalIndent(todo, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal todo: %w", err)
			}
			fmt.Println(string(todoJSON))
			return nil
		}

		if err := worker.ExecuteWork(ctx, todo); err != nil {
			return fmt.Errorf("failed to execute work: %w", err)
		}

		return nil
	},
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

	var ri librelease.ReleaseInfo
	releaseRef, err := refs.Reduce(parsedResolvedDeployment.String(), librelease.GlobRelease)
	if err != nil {
		return nil, fmt.Errorf("failed to reduce ref: %w", err)
	}
	if err := store.Get(ctx, releaseRef, &ri); err != nil {
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

func init() {
	RootCmd.AddCommand(WorkCmd)

	WorkContinueCmd.Flags().BoolP("dryrun", "d", false, "List refs for work that would be triggered")
	WorkCmd.AddCommand(WorkContinueCmd)

	WorkTriggerCommand.Flags().BoolP("dryrun", "d", false, "List refs for work that would be triggered")
	WorkTriggerCommand.Flags().BoolP("intent", "i", false, "Trigger intents instead of deployments")
	WorkCmd.AddCommand(WorkTriggerCommand)

	WorkOpsCmd.Flags().BoolP("dryrun", "d", false, "List refs for work that would be triggered")
	WorkCmd.AddCommand(WorkOpsCmd)

	WorkAnyCommand.Flags().BoolP("dryrun", "d", false, "List refs for work that would be triggered")
	WorkCmd.AddCommand(WorkAnyCommand)
}
