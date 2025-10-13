package work

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client"
	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/client/tui"
	"github.com/ocuroot/ocuroot/client/tui/tuiwork"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/store/models"

	librelease "github.com/ocuroot/ocuroot/lib/release"
)

func NewWorker(ctx context.Context, ref refs.Ref) (*Worker, error) {
	workTui := tui.StartWorkTui()

	w := &Worker{
		Tui: workTui,

		StateChanges:  make(map[string]struct{}),
		IntentChanges: make(map[string]struct{}),
	}

	wd, err := os.Getwd()
	if err != nil {
		workTui.Cleanup()
		return nil, err
	}
	w.RepoInfo, err = client.GetRepoInfo(wd)
	if err != nil {
		workTui.Cleanup()
		return nil, err
	}

	if w.RepoInfo.IsSource {
		err := w.InitTrackerFromSourceRepo(ctx, ref, wd, w.RepoInfo.Root, true)
		if err != nil {
			workTui.Cleanup()
			return nil, fmt.Errorf("failed to init tracker: %w", err)
		}
	} else {
		err = w.InitTrackerFromStateRepo(ctx, ref, wd, w.RepoInfo.Root)
		if err != nil {
			workTui.Cleanup()
			return nil, fmt.Errorf("failed to init tracker from state repo: %w", err)
		}
	}

	w.Tracker.State = tuiwork.WatchForStateUpdates(ctx, w.Tracker.State, workTui)
	w.RecordStateUpdates(ctx)

	return w, nil
}

type Worker struct {
	Tracker     release.TrackerConfig
	RepoName    string
	RepoRemotes []string
	RepoInfo    client.RepoInfo

	Tui tui.Tui

	StateChanges  map[string]struct{}
	IntentChanges map[string]struct{}
}

type GitFilter int

const (
	GitFilterNone GitFilter = iota
	GitFilterCurrentRepoOnly
	GitFilterCurrentCommitOnly
)

type IdentifyWorkRequest struct {
	// Filter work based on the repo and commit required
	GitFilter GitFilter

	// Filter work based on upstream changes that impact it
	// This allows work to apply a release to continue through
	// other releases with dependencies, for example
	IntentChanges map[string]struct{}
	StateChanges  map[string]struct{}
}

func (w *Worker) Cleanup() {
	w.Tui.Cleanup()
}

func (w *Worker) IdentifyWork(ctx context.Context, req IdentifyWorkRequest) ([]Work, error) {
	var out []Work

	diffs, err := w.Diff(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}
	out = append(out, diffs...)

	readyRuns, err := w.ReadyRuns(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get ready runs: %w", err)
	}
	out = append(out, readyRuns...)

	reconcilableDeployments, err := w.ReconcilableDeployments(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get reconcilable deployments: %w", err)
	}
	out = append(out, reconcilableDeployments...)

	ops, err := w.Ops(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get ops: %w", err)
	}
	out = append(out, ops...)

	return out, nil
}

func (w *Worker) ExecuteWork(ctx context.Context, todos []Work) error {
	log.Info("Applying intent diffs")
	for _, t := range todos {
		if t.WorkType == WorkTypeUpdate || t.WorkType == WorkTypeCreate || t.WorkType == WorkTypeDelete {
			if err := w.ApplyIntent(ctx, t.Ref); err != nil {
				return fmt.Errorf("failed to apply intent (%s): %w", t.Ref.String(), err)
			}
		}
	}

	log.Info("Starting op work")
	for _, t := range todos {
		if t.WorkType == WorkTypeOp {
			if err := w.runOp(ctx, t.Ref.String()); err != nil {
				return fmt.Errorf("failed to run op (%s): %w", t.Ref.String(), err)
			}
		}
	}

	log.Info("Executing releases")
	for _, t := range todos {
		if t.WorkType == WorkTypeRelease {
			if err := w.startRelease(ctx, t.Ref); err != nil {
				return fmt.Errorf("failed to start release (%s): %w", t.Ref.String(), err)
			}
		}
	}

	log.Info("Starting release work")
	for _, t := range todos {
		if t.WorkType == WorkTypeRun {
			if t.Ref.SubPathType == refs.SubPathTypeDeploy {
				if err := w.addRunForDeployment(ctx, t.Ref.String()); err != nil {
					// If a run is not needed this will error out because the deploy isn't there
					log.Info("failed to add run for deployment", "ref", t.Ref.String(), "error", err)
				}
			}
			releaseRef, err := refs.Reduce(t.Ref.String(), librelease.GlobRelease)
			if err != nil {
				return fmt.Errorf("failed to reduce ref: %w", err)
			}
			pr, err := refs.Parse(releaseRef)
			if err != nil {
				return fmt.Errorf("failed to parse ref: %w", err)
			}
			w.Tracker.Ref = pr
			if err := w.continueRelease(ctx, w.Tui); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *Worker) runOp(ctx context.Context, ref string) error {
	var err error

	w.Tracker.Ref, err = refs.Parse(ref)
	if err != nil {
		return fmt.Errorf("failed to parse ref: %w", err)
	}
	w.Tracker.Ref.SubPath = ""
	w.Tracker.Ref.SubPathType = refs.SubPathTypeNone

	log.Info("Setting up tracker", "tc", w.Tracker)
	tracker, err := w.TrackerForExistingRelease(ctx)
	if err != nil {
		if errors.Is(err, refstore.ErrRefNotFound) {
			log.Error("The specified release was not found", "ref", w.Tracker.Ref.String())
			return nil
		}
		return fmt.Errorf("failed to get tracker: %w", err)
	}

	err = tracker.Op(ctx, ref, nil)
	if err != nil {
		return fmt.Errorf("running task in tracker: %w", err)
	}

	return nil
}

func (w *Worker) continueRelease(ctx context.Context, workTui tui.Tui) error {
	tracker, err := w.TrackerForExistingRelease(ctx)
	if err != nil {
		if errors.Is(err, refstore.ErrRefNotFound) {
			log.Error("The specified release was not found", "ref", w.Tracker.Ref.String())
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

func (w *Worker) addRunForDeployment(ctx context.Context, ref string) error {
	state := w.Tracker.State

	ref, err := refs.Reduce(ref, librelease.GlobDeployment)
	if err != nil {
		return fmt.Errorf("failed to reduce ref: %w", err)
	}
	log.Info("Adding run for deployment if needed", "ref", ref)
	var deployment models.Task
	if err := state.Get(ctx, ref, &deployment); err != nil {
		return fmt.Errorf("failed to get deployment %q: %w", ref, err)
	}
	var run models.Run
	if err := state.Get(ctx, deployment.RunRef.String(), &run); err != nil {
		return fmt.Errorf("failed to get run %q: %w", deployment.RunRef.String(), err)
	}

	resolvedDeployment, err := state.ResolveLink(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to resolve inputs %q: %w", ref, err)
	}
	parsedResolvedTask, err := refs.Parse(resolvedDeployment)
	if err != nil {
		return fmt.Errorf("failed to parse resolved deployment %q: %w", ref, err)
	}

	var release librelease.ReleaseInfo
	if err := state.Get(ctx, parsedResolvedTask.SetSubPathType(refs.SubPathTypeNone).SetSubPath("").SetFragment("").String(), &release); err != nil {
		return fmt.Errorf("failed to get release %q: %w", ref, err)
	}

	entryFunctionInputs := deployment.Inputs
	inputs, err := librelease.PopulateInputs(ctx, state, entryFunctionInputs)
	if err != nil {
		return fmt.Errorf("failed to populate inputs %q: %w", ref, err)
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
		return nil
	}

	log.Info("Duplicating deployment", "ref", resolvedDeployment)
	parsedResolvedTask, err = refs.Parse(resolvedDeployment)
	if err != nil {
		return fmt.Errorf("failed to parse resolved deployment %q: %w", ref, err)
	}
	newRunRefString, err := refstore.IncrementPath(ctx, state, fmt.Sprintf("%s/", parsedResolvedTask.String()))
	if err != nil {
		return fmt.Errorf("failed to increment path %q: %w", ref, err)
	}
	log.Info("Incremented path", "ref", ref, "newRef", newRunRefString)
	newRunRef, err := refs.Parse(newRunRefString)
	if err != nil {
		return fmt.Errorf("failed to parse path %q: %w", ref, err)
	}
	err = librelease.InitializeRun(ctx, state, newRunRef, &models.Function{
		Fn:     run.Functions[0].Fn,
		Inputs: inputs,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize run %q: %w", ref, err)
	}

	log.Info("Duplicated deployment", "oldRef", resolvedDeployment, "newRef", newRunRef)
	return nil
}
