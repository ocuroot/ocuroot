package release

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"
	"github.com/ocuroot/ocuroot/ui/components/pipeline"
)

// ReleaseStore creates a ReleaseStore from a string ref.
// The ref may be a link, or even a ref to a deploy or function within a release.
// It resolves the ref, removes the subpath/fragment, and creates a wrapped store.
func ReleaseStore(ctx context.Context, ref string, store refstore.Store) (*releaseStore, error) {
	resolvedRefStr, err := store.ResolveLink(ctx, ref)
	if err != nil {
		return nil, err
	}
	releaseRef, err := refs.Parse(resolvedRefStr)
	if err != nil {
		return nil, err
	}
	releaseRef = releaseRef.SetSubPath("").SetSubPathType(refs.SubPathTypeNone).SetFragment("")
	return &releaseStore{
		ReleaseRef: releaseRef,
		Store:      store,
	}, nil
}

type releaseStore struct {
	ReleaseRef refs.Ref
	Store      refstore.Store
}

func (r *releaseStore) GetReleaseInfo() (*ReleaseInfo, error) {
	var releaseInfo ReleaseInfo
	err := r.Store.Get(context.Background(), r.ReleaseRef.String(), &releaseInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to get release state: %w", err)
	}
	return &releaseInfo, nil
}

func (w *releaseStore) InitDeployment(ctx context.Context, env string, up bool) error {
	err := w.Store.StartTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		commitErr := w.Store.CommitTransaction(ctx, "initializing deployment")
		if commitErr != nil {
			log.Error("failed to commit transaction", "error", commitErr)
		}
	}()

	ri, err := w.GetReleaseInfo()
	if err != nil {
		return fmt.Errorf("failed to get release state: %w", err)
	}

	var work *sdk.Work
	for _, phase := range ri.Package.Phases {
		for _, w := range phase.Work {
			if w.Deployment != nil && w.Deployment.Environment == sdk.EnvironmentName(env) {
				work = &w
				break
			}
		}
		if work != nil {
			break
		}
	}
	if work == nil {
		return fmt.Errorf("release is not configured for environment %s", env)
	}

	ref, fnWork, fs, err := w.SDKWorkToFunctionChain(ctx, *work, up)
	if err != nil {
		return fmt.Errorf("failed to get function chain: %w", err)
	}
	err = w.InitializeFunction(ctx, fnWork, ref, fs)
	if err != nil {
		return fmt.Errorf("failed to initialize function: %w", err)
	}
	return nil
}

func FunctionIsReady(ctx context.Context, store refstore.Store, ref string) (bool, error) {
	functionRef, err := refs.Reduce(ref, GlobFunction)
	if err != nil {
		return false, fmt.Errorf("failed to reduce ref: %w", err)
	}

	var functionState FunctionState
	if err := store.Get(ctx, functionRef, &functionState); err != nil {
		return false, fmt.Errorf("failed to get function state at %s: %w", functionRef, err)
	}

	dependenciesSatisfied, err := CheckDependencies(ctx, store, functionState)
	if err != nil {
		return false, fmt.Errorf("failed to check dependencies: %w", err)
	}

	if !dependenciesSatisfied {
		return false, nil
	}

	inputs, err := PopulateInputs(ctx, store, functionState.Current.Inputs)
	if err != nil {
		return false, fmt.Errorf("failed to populate inputs: %w", err)
	}

	for _, input := range inputs {
		if input.Value == nil && input.Default == nil {
			return false, nil
		}
	}

	return true, nil
}

func CheckDependencies(ctx context.Context, store refstore.Store, fs FunctionState) (bool, error) {
	if len(fs.Current.Dependencies) == 0 {
		return true, nil
	}

	var deps []string
	for _, dep := range fs.Current.Dependencies {
		deps = append(deps, dep.String())
	}
	matchedDeps, err := store.Match(ctx, deps...)
	if err != nil {
		return false, err
	}
	if len(matchedDeps) != len(fs.Current.Dependencies) {
		return false, nil
	}

	return true, nil
}

func (w *releaseStore) PendingFunctions(ctx context.Context) (map[refs.Ref]*models.Function, error) {
	matchRef := w.ReleaseRef.String() + "/**/functions/*/status/pending"
	pendingFunctions, err := w.Store.Match(ctx, matchRef)
	if err != nil {
		return nil, err
	}

	out := make(map[refs.Ref]*models.Function)
	for _, fn := range pendingFunctions {
		functionRef := strings.TrimSuffix(fn, "/status/pending")

		var function FunctionState
		err := w.Store.Get(ctx, functionRef, &function)
		if err != nil {
			return nil, err
		}
		// TODO: There should be a clearer indicator
		if function.Current.Fn.Name == "" {
			return nil, fmt.Errorf("function %s not found", functionRef)
		}

		// Check that all dependencies are satisfied
		satisfied, err := CheckDependencies(ctx, w.Store, function)
		if err != nil {
			return nil, err
		}
		if !satisfied {
			log.Info("function dependencies not satisfied", "function", functionRef, "dependencies", function.Current.Dependencies)
			continue
		}

		fr, err := refs.Parse(functionRef)
		if err != nil {
			return nil, err
		}
		out[fr] = &function.Current
	}
	return out, nil
}

// AddTags implements ReleaseStateStore.
func (w *releaseStore) AddTags(ctx context.Context, tags []string) error {
	for _, tag := range tags {
		tagRef := w.ReleaseRef
		tagRef.ReleaseOrIntent = refs.ReleaseOrIntent{
			Type:  refs.Release,
			Value: tag,
		}
		if err := w.Store.Link(ctx, tagRef.String(), w.ReleaseRef.String()); err != nil {
			return err
		}
	}
	return nil
}

// GetReleaseState implements ReleaseStateStore.
func (w *releaseStore) GetReleaseState(ctx context.Context) (*pipeline.ReleaseSummary, error) {
	var release pipeline.ReleaseSummary
	err := w.Store.Get(ctx, w.ReleaseRef.String(), &release)
	if err != nil {
		return nil, err
	}
	return &release, nil
}

func GetFunctionChainStatusFromFunctions(ctx context.Context, store refstore.Store, chainRef refs.Ref) (models.Status, error) {
	resultMatches, err := store.Match(ctx, chainRef.String()+"/functions/*/status/*")
	if err != nil {
		return "", err
	}

	var statusCounts map[string]int
	for _, match := range resultMatches {
		status := path.Base(match)
		if statusCounts == nil {
			statusCounts = make(map[string]int)
		}
		statusCounts[status]++
	}

	if statusCounts[string(models.StatusPending)] == len(resultMatches) {
		return models.StatusPending, nil
	}
	if statusCounts[string(models.StatusComplete)] == len(resultMatches) {
		return models.StatusComplete, nil
	}

	if statusCounts[string(models.StatusRunning)] > 0 {
		return models.StatusRunning, nil
	}
	if statusCounts[string(models.StatusFailed)] > 0 {
		return models.StatusFailed, nil
	}
	if statusCounts[string(models.StatusCancelled)] > 0 {
		return models.StatusCancelled, nil
	}

	if statusCounts[string(models.StatusPending)] > 0 && statusCounts[string(models.StatusComplete)] > 0 {
		return models.StatusRunning, nil
	}

	// Default to pending
	return models.StatusPending, nil
}

func GetFunctionChainStatus(ctx context.Context, store refstore.Store, chainRef refs.Ref) (models.Status, error) {
	resultMatches, err := store.Match(ctx, chainRef.String()+"/status/*")
	if err != nil {
		return "", err
	}

	if len(resultMatches) == 0 {
		return models.StatusPending, nil
	}
	if len(resultMatches) > 1 {
		return "", fmt.Errorf("expected 1 result, got %d (%v)", len(resultMatches), resultMatches)
	}

	return models.Status(path.Base(resultMatches[0])), nil
}

func (w *releaseStore) GetFunctionChainStatus(ctx context.Context, chainRef refs.Ref) (models.Status, error) {
	return GetFunctionChainStatus(ctx, w.Store, chainRef)
}

func (w *releaseStore) InitializeFunction(
	ctx context.Context,
	workState models.Work,
	functionChainRef refs.Ref,
	fn *models.Function,
) error {
	// Set the initial (pending) state for the function
	functionRef := FunctionRefFromChainRef(functionChainRef, fn)
	err := UpdateFunctionStateUnderRef(ctx, w.Store, functionRef, fn)
	if err != nil {
		return fmt.Errorf("failed to update function state: %w", err)
	}

	if workState.Entrypoint.String() == "" {
		workState.Entrypoint = functionRef
	}

	if err := w.Store.Set(ctx, functionChainRef.String(), workState); err != nil {
		return fmt.Errorf("failed to set work state: %w", err)
	}

	if err := saveStatus(ctx, w.Store, functionChainRef, models.StatusPending); err != nil {
		return fmt.Errorf("failed to save status: %w", err)
	}

	return nil
}

func InitializeFunctionChain(
	ctx context.Context,
	store refstore.Store,
	functionChainRef refs.Ref,
	fn *models.Function,
) error {
	// Set the initial (pending) state for the function
	functionRef := FunctionRefFromChainRef(functionChainRef, fn)
	err := UpdateFunctionStateUnderRef(ctx, store, functionRef, fn)
	if err != nil {
		return fmt.Errorf("failed to update function state: %w", err)
	}

	releaseRef := functionChainRef.SetFragment("").SetSubPath("").SetSubPathType(refs.SubPathTypeNone)

	workState := models.Work{
		Release:    releaseRef,
		Entrypoint: functionRef,
	}
	if err := store.Set(ctx, functionChainRef.String(), workState); err != nil {
		return fmt.Errorf("failed to set work state: %w", err)
	}

	if err := saveStatus(ctx, store, functionChainRef, models.StatusPending); err != nil {
		return fmt.Errorf("failed to save status: %w", err)
	}
	return nil
}

func UpdateFunctionStateUnderRef(ctx context.Context, store refstore.Store, functionRef refs.Ref, function *models.Function) error {
	functionStatusRef := functionRef.JoinSubPath(statusPathSegment)

	var s FunctionState
	if err := store.Get(ctx, functionStatusRef.String(), &s); err != nil && !errors.Is(err, refstore.ErrRefNotFound) {
		return fmt.Errorf("failed to get function state: %w", err)
	}

	s.Current = *function
	s.History = append(s.History, StatusEvent{
		Time:   time.Now(),
		Status: function.Status,
	})

	if err := store.Set(ctx, functionRef.String(), s); err != nil {
		return fmt.Errorf("failed to set function state: %w", err)
	}

	if err := saveStatus(ctx, store, functionRef, function.Status); err != nil {
		return err
	}

	return nil
}

func saveStatus(ctx context.Context, store refstore.Store, ref refs.Ref, status models.Status) error {
	functionStatusRef := ref.JoinSubPath(statusPathSegment)

	// Remove any existing status markers
	existingStatuses, err := store.Match(ctx, functionStatusRef.String()+"/*")
	if err != nil {
		return fmt.Errorf("failed to match function status: %w", err)
	}
	for _, status := range existingStatuses {
		store.Delete(ctx, status)
	}

	t := time.Now()
	functionStateRef := functionStatusRef.JoinSubPath(string(status))
	if err := store.Set(ctx, functionStateRef.String(), StatusMarker{Time: t}); err != nil {
		return fmt.Errorf("failed to set function state: %w", err)
	}

	return nil
}

type FunctionState struct {
	Current models.Function `json:"current"`
	History []StatusEvent   `json:"history"`
}

type StatusEvent struct {
	Time   time.Time     `json:"time"`
	Status models.Status `json:"status"`
}

type StatusMarker struct {
	Time time.Time `json:"time"`
}

const (
	statusPathSegment    = "status"
	functionsPathSegment = "functions"
)

func (r *releaseStore) SDKWorkToFunctionChain(ctx context.Context, work sdk.Work, down bool) (refs.Ref, models.Work, *models.Function, error) {
	workRef := r.ReleaseRef

	mWork := models.Work{
		Release: r.ReleaseRef,
	}
	fs := &models.Function{
		ID:     "1",
		Status: models.StatusPending,
	}

	if work.Deployment != nil {
		workRef = workRef.
			SetSubPathType(refs.SubPathTypeDeploy).
			SetSubPath(string(work.Deployment.Environment))
		if down {
			mWork.Type = models.WorkTypeDown
			fs.Fn = work.Deployment.Down
		} else {
			mWork.Type = models.WorkTypeUp
			fs.Fn = work.Deployment.Up
		}
		fs.Inputs = work.Deployment.Inputs
	}

	if work.Call != nil {
		workRef = workRef.
			SetSubPathType(refs.SubPathTypeCall).
			SetSubPath(work.Call.Name)
		mWork.Type = models.WorkTypeCall
		fs.Fn = work.Call.Fn
		fs.Inputs = work.Call.Inputs
	}

	workRefString, err := refstore.IncrementPath(ctx, r.Store, fmt.Sprintf("%s/", workRef.String()))
	if err != nil {
		return refs.Ref{}, models.Work{}, nil, fmt.Errorf("failed to increment path: %w", err)
	}
	workRef, err = refs.Parse(workRefString)
	if err != nil {
		return refs.Ref{}, models.Work{}, nil, fmt.Errorf("failed to parse path: %w", err)
	}

	return workRef, mWork, fs, nil
}

// SDKPackageToFunctionChains converts a SDK package to a map of function chain refs to
// function summaries representing the first function in each chain
func (r *releaseStore) SDKPackageToFunctionChains(ctx context.Context, pkg *sdk.Package) (map[refs.Ref]*models.Function, error) {
	functionChains := make(map[refs.Ref]*models.Function)
	var previousPhaseRefs []refs.Ref
	for _, phase := range pkg.Phases {
		var currentPhaseRefs []refs.Ref
		for _, work := range phase.Work {
			workRef, _, fc, err := r.SDKWorkToFunctionChain(ctx, work, false)
			if err != nil {
				return nil, err
			}
			fc.Dependencies = previousPhaseRefs
			functionChains[workRef] = fc
			currentPhaseRefs = append(currentPhaseRefs, workRef.JoinSubPath("status", string(models.StatusComplete)))
		}
		previousPhaseRefs = currentPhaseRefs
	}
	return functionChains, nil
}
