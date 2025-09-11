package work

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/ocuroot/ocuroot/lib/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/store/models"
)

func (w *Worker) ReadyRuns(ctx context.Context, req IndentifyWorkRequest) ([]Work, error) {
	var out []Work

	state := w.Tracker.State

	prefix := "**"
	if req.GitFilter == GitFilterCurrentRepoOnly || req.GitFilter == GitFilterCurrentCommitOnly {
		prefix = fmt.Sprintf("%s/-/**", w.Tracker.Ref.Repo)
	}

	// Match any outstanding runs in the state repo
	outstanding, err := state.Match(
		ctx,
		prefix+"/@*/{deploy,task}/*/*/status/pending",
		prefix+"/@*/{deploy,task}/*/*/status/paused",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to match refs: %w", err)
	}

	for _, ref := range outstanding {
		// Reduce the status ref back to the run ref
		runRef, err := refs.Reduce(ref, release.GlobRun)
		if err != nil {
			return nil, fmt.Errorf("failed to reduce ref: %w", err)
		}

		// Filter by commit as needed
		valid, err := w.CheckCommit(ctx, runRef, req)
		if err != nil {
			return nil, fmt.Errorf("failed to check commit: %w", err)
		}
		if !valid {
			continue
		}

		// Check if the run is ready
		funcReady, err := release.RunIsReady(ctx, state, runRef)
		if err != nil {
			return nil, fmt.Errorf("failed to check run: %w", err)
		}
		if !funcReady {
			continue
		}

		rp, err := refs.Parse(runRef)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ref: %w", err)
		}
		out = append(out, Work{
			Ref:      rp,
			WorkType: WorkTypeRun,
		})
	}

	return out, nil
}

func (w *Worker) CheckCommit(ctx context.Context, ref string, req IndentifyWorkRequest) (bool, error) {
	if req.GitFilter != GitFilterCurrentCommitOnly {
		return true, nil
	}

	resolvedRef, err := w.Tracker.State.ResolveLink(ctx, ref)
	if err != nil {
		return false, fmt.Errorf("failed to resolve ref: %w", err)
	}

	releaseRef, err := refs.Reduce(resolvedRef, release.GlobRelease)
	if err != nil {
		return false, fmt.Errorf("failed to reduce ref: %w", err)
	}
	var r release.ReleaseInfo
	if err := w.Tracker.State.Get(ctx, releaseRef, &r); err != nil {
		return false, fmt.Errorf("failed to get release %q: %w", ref, err)
	}
	return r.Commit == w.Tracker.Commit, nil
}

func (w *Worker) ReconcilableDeployments(ctx context.Context, req IndentifyWorkRequest) ([]Work, error) {
	var out []Work

	// Identify any reconcilable deployments (where inputs have changed)
	reconcilable, err := w.getReconcilableDeployments(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get reconcilable deployments: %w", err)
	}

	for _, ref := range reconcilable {
		out = append(out, Work{
			Ref:      ref,
			WorkType: WorkTypeRun,
		})
	}

	return out, nil
}

func (w *Worker) Ops(ctx context.Context, req IndentifyWorkRequest) ([]Work, error) {
	var out []Work

	state := w.Tracker.State
	// Identify any ops

	prefix := "**"
	if req.GitFilter == GitFilterCurrentRepoOnly || req.GitFilter == GitFilterCurrentCommitOnly {
		prefix = fmt.Sprintf("%s/-/**", w.Tracker.Ref.Repo)
	}
	g := prefix + "/@*/op/*"
	log.Info("Identifying ops", "glob", g)
	outstanding, err := state.Match(
		ctx,
		g,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to match refs: %w", err)
	}

	for _, ref := range outstanding {
		parsedRef, err := refs.Parse(ref)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ref: %w", err)
		}
		log.Info("Found outstanding op", "ref", parsedRef.String())
		out = append(out, Work{
			Ref:      parsedRef,
			WorkType: WorkTypeOp,
		})
	}

	return out, nil
}

func (w *Worker) getReconcilableDeployments(ctx context.Context, req IndentifyWorkRequest) ([]refs.Ref, error) {
	var out []refs.Ref

	log.Info("Getting reconcilable deployments")

	prefix := "**"
	if req.GitFilter == GitFilterCurrentRepoOnly || req.GitFilter == GitFilterCurrentCommitOnly {
		prefix = fmt.Sprintf("%s/-/**", w.Tracker.Ref.Repo)
	}
	allDeployments, err := w.Tracker.State.Match(ctx, prefix+"/@/deploy/*")
	if err != nil {
		return nil, fmt.Errorf("failed to match deployments: %w", err)
	}

	for _, ref := range allDeployments {
		valid, err := w.CheckCommit(ctx, ref, req)
		if err != nil {
			return nil, fmt.Errorf("failed to check commit: %w", err)
		}
		if !valid {
			continue
		}

		resolved, err := reconcileDeployment(ctx, w.Tracker.State, ref)
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

	entryFunction := run.Functions[0]
	dependenciesSatisfied, err := release.CheckDependencies(ctx, store, entryFunction)
	if err != nil {
		return nil, fmt.Errorf("failed to check dependencies: %w", err)
	}
	// Don't attempt to trigger if the dependencies haven't been satisfied
	if !dependenciesSatisfied {
		return nil, nil
	}

	entryFunctionInputs := entryFunction.Inputs
	inputs, err := release.PopulateInputs(ctx, store, entryFunctionInputs)
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

func toJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}
