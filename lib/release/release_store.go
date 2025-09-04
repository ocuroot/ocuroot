package release

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
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

func (r *releaseStore) GetReleaseInfo(ctx context.Context) (*ReleaseInfo, error) {
	var releaseInfo ReleaseInfo
	err := r.Store.Get(ctx, r.ReleaseRef.String(), &releaseInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to get release state: %w", err)
	}
	return &releaseInfo, nil
}

func (w *releaseStore) InitDeploymentUp(ctx context.Context, env string) error {
	err := w.Store.StartTransaction(ctx, "initializing deployment")
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		commitErr := w.Store.CommitTransaction(ctx)
		if commitErr != nil {
			log.Error("failed to commit transaction", "error", commitErr)
		}
	}()

	ri, err := w.GetReleaseInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get release state: %w", err)
	}

	var work *sdk.Task
	for _, phase := range ri.Package.Phases {
		for _, w := range phase.Tasks {
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

	ref, fnWork, fs, err := w.sdkWorkToFunctionChain(ctx, *work)
	if err != nil {
		return fmt.Errorf("failed to get function chain: %w", err)
	}
	err = w.InitializeFunction(ctx, fnWork, ref, fs)
	if err != nil {
		return fmt.Errorf("failed to initialize function: %w", err)
	}
	return nil
}

func (w *releaseStore) InitDeploymentDown(ctx context.Context, env string) error {
	// Get the current deployment
	currentDeploymentRef := w.ReleaseRef.SetSubPathType(refs.SubPathTypeDeploy).SetSubPath(env)
	currentDeployment := models.Task{}
	if err := w.Store.Get(ctx, currentDeploymentRef.String(), &currentDeployment); err != nil {
		if errors.Is(err, refstore.ErrRefNotFound) {
			log.Info("no current deployment found. nothing to be done", "environment", env)
			return nil
		}
		return fmt.Errorf("failed to get current deployment: %w", err)
	}
	var run models.Run
	if err := w.Store.Get(ctx, currentDeployment.RunRef.String(), &run); err != nil {
		return fmt.Errorf("failed to get run at %s: %w", currentDeployment.RunRef.String(), err)
	}

	entrypoint := run.Functions[0]

	ri, err := w.GetReleaseInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get release state: %w", err)
	}

	var downFunc *sdk.FunctionDef
	for _, phase := range ri.Package.Phases {
		for _, w := range phase.Tasks {
			if w.Deployment != nil && w.Deployment.Environment == sdk.EnvironmentName(env) {
				downFunc = &w.Deployment.Down
				break
			}
		}
		if downFunc != nil {
			break
		}
	}
	if downFunc == nil {
		return fmt.Errorf("release has no down function %s", env)
	}

	err = w.Store.StartTransaction(ctx, "initializing deployment (down)")
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		commitErr := w.Store.CommitTransaction(ctx)
		if commitErr != nil {
			log.Error("failed to commit transaction", "error", commitErr)
		}
	}()

	ref, fnWork, fs, err := w.sdkWorkToFunctionChainDown(ctx, env, entrypoint, *downFunc)
	if err != nil {
		return fmt.Errorf("failed to get function chain: %w", err)
	}

	err = w.InitializeFunction(ctx, fnWork, ref, fs)
	if err != nil {
		return fmt.Errorf("failed to initialize function: %w", err)
	}
	return nil
}

func JobIsReady(ctx context.Context, store refstore.Store, ref string) (bool, error) {
	jobRef, err := refs.Reduce(ref, GlobRun)
	if err != nil {
		return false, fmt.Errorf("failed to reduce ref: %w", err)
	}

	var work models.Run
	if err := store.Get(ctx, jobRef, &work); err != nil {
		return false, fmt.Errorf("failed to get function state at %s: %w", jobRef, err)
	}
	if len(work.Functions) == 0 {
		return false, fmt.Errorf("no functions in work")
	}
	lastFunction := work.Functions[len(work.Functions)-1]
	dependenciesSatisfied, err := CheckDependencies(ctx, store, lastFunction)
	if err != nil {
		return false, fmt.Errorf("failed to check dependencies: %w", err)
	}

	if !dependenciesSatisfied {
		return false, nil
	}

	inputs, err := PopulateInputs(ctx, store, lastFunction.Inputs)
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

func CheckDependencies(ctx context.Context, store refstore.Store, fn *models.Function) (bool, error) {
	if len(fn.Dependencies) == 0 {
		return true, nil
	}

	var deps []string
	for _, dep := range fn.Dependencies {
		deps = append(deps, dep.String())
	}
	matchedDeps, err := store.Match(ctx, deps...)
	if err != nil {
		return false, err
	}
	if len(matchedDeps) != len(fn.Dependencies) {
		return false, nil
	}

	return true, nil
}

func (w *releaseStore) FailedJobs(ctx context.Context) (map[refs.Ref]*models.Run, error) {
	matchRef := w.ReleaseRef.String() + "/{task,deploy}/*/*/status/failed"
	failedJobs, err := w.Store.Match(ctx, matchRef)
	if err != nil {
		return nil, err
	}

	out := make(map[refs.Ref]*models.Run)
	for _, fn := range failedJobs {
		jobRef := strings.TrimSuffix(fn, "/status/failed")

		var work models.Run
		err := w.Store.Get(ctx, jobRef, &work)
		if err != nil {
			return nil, err
		}

		fr, err := refs.Parse(jobRef)
		if err != nil {
			return nil, err
		}
		out[fr] = &work
	}
	return out, nil
}

func (w *releaseStore) PendingJobs(ctx context.Context) (map[refs.Ref]*models.Run, error) {
	matchRefPending := w.ReleaseRef.String() + "/{task,deploy}/*/*/status/pending"
	matchRefPaused := w.ReleaseRef.String() + "/{task,deploy}/*/*/status/paused"

	log.Info("PendingJobs globs", "pending", matchRefPending, "paused", matchRefPaused)

	pendingJobs, err := w.Store.Match(ctx, matchRefPending, matchRefPaused)
	if err != nil {
		return nil, err
	}

	out := make(map[refs.Ref]*models.Run)
	for _, fn := range pendingJobs {
		workRef, err := refs.Reduce(fn, GlobRun)
		if err != nil {
			return nil, err
		}

		var work models.Run
		err = w.Store.Get(ctx, workRef, &work)
		if err != nil {
			return nil, err
		}
		function := work.Functions[len(work.Functions)-1]

		// TODO: There should be a clearer indicator
		if function.Fn.Name == "" {
			return nil, fmt.Errorf("function %s not found", workRef)
		}

		// Check that all dependencies are satisfied
		satisfied, err := CheckDependencies(ctx, w.Store, function)
		if err != nil {
			return nil, err
		}
		if !satisfied {
			log.Info("function dependencies not satisfied", "function", workRef, "dependencies", function.Dependencies)
			continue
		}

		workRefParsed, err := refs.Parse(workRef)
		if err != nil {
			return nil, err
		}

		out[workRefParsed] = &work
	}
	return out, nil
}

var (
	TagRegex        = regexp.MustCompile(`^[a-zA-Z0-9-_\.]+$`)
	TagReleaseRegex = regexp.MustCompile(`^r[0-9]+$`)
)

// AddTags implements ReleaseStateStore.
func (w *releaseStore) AddTags(ctx context.Context, tags []string) error {
	for _, tag := range tags {
		if !TagRegex.MatchString(tag) {
			return fmt.Errorf("invalid tag: %s", tag)
		}
		if TagReleaseRegex.MatchString(tag) {
			return fmt.Errorf("tags must not be in the release format (r<number>): %s", tag)
		}
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

func GetWorkStatus(ctx context.Context, store refstore.Store, workRef refs.Ref) (models.Status, error) {
	resultMatches, err := store.Match(ctx, workRef.String()+"/status/*")
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

func (w *releaseStore) GetWorkStatus(ctx context.Context, workRef refs.Ref) (models.Status, error) {
	return GetWorkStatus(ctx, w.Store, workRef)
}

var validInputNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func validateFunction(fn *models.Function) error {
	for inputName := range fn.Inputs {
		if !validInputNameRegex.MatchString(inputName) {
			return fmt.Errorf("invalid input name %q", inputName)
		}
	}
	return nil
}

func (w *releaseStore) InitializeFunction(
	ctx context.Context,
	workState models.Run,
	functionChainRef refs.Ref,
	fn *models.Function,
) error {
	log.Info("Initializing function", "ref", functionChainRef.String())

	// Validate contents of function
	if err := validateFunction(fn); err != nil {
		return err
	}

	workState.Functions = append(workState.Functions, fn)

	if err := w.Store.Set(ctx, functionChainRef.String(), workState); err != nil {
		return fmt.Errorf("failed to set work state: %w", err)
	}

	if err := saveStatus(ctx, w.Store, functionChainRef, models.StatusPending); err != nil {
		return fmt.Errorf("failed to save status: %w", err)
	}

	return nil
}

func InitializeRun(
	ctx context.Context,
	store refstore.Store,
	runRef refs.Ref,
	fn *models.Function,
) error {
	releaseRef := runRef.SetFragment("").SetSubPath("").SetSubPathType(refs.SubPathTypeNone)

	workState := models.Run{
		Release: releaseRef,
		Functions: []*models.Function{
			fn,
		},
	}
	if runRef.SubPathType == refs.SubPathTypeTask {
		workState.Type = models.JobTypeTask
	}
	if runRef.SubPathType == refs.SubPathTypeDeploy {
		workState.Type = models.JobTypeUp
	}

	log.Info("Initializing run", "ref", runRef.String(), "runState", workState)
	if err := store.Set(ctx, runRef.String(), workState); err != nil {
		return fmt.Errorf("failed to set run state: %w", err)
	}

	log.Info("Saving status", "ref", runRef.String(), "status", models.StatusPending)
	if err := saveStatus(ctx, store, runRef, models.StatusPending); err != nil {
		return fmt.Errorf("failed to save status: %w", err)
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

	functionStateRef := functionStatusRef.JoinSubPath(string(status))
	if err := store.Set(ctx, functionStateRef.String(), models.NewMarker()); err != nil {
		return fmt.Errorf("failed to set function state: %w", err)
	}

	log.Info("saved status", "status", status, "ref", ref.String(), "fsr", functionStateRef.String())

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

const (
	statusPathSegment = "status"
)

func (r *releaseStore) sdkWorkToFunctionChain(ctx context.Context, work sdk.Task) (refs.Ref, models.Run, *models.Function, error) {
	workRef := r.ReleaseRef

	mWork := models.Run{
		Release: r.ReleaseRef,
	}
	fs := &models.Function{}

	if work.Deployment != nil {
		workRef = workRef.
			SetSubPathType(refs.SubPathTypeDeploy).
			SetSubPath(string(work.Deployment.Environment))
		mWork.Type = models.JobTypeUp
		fs.Fn = work.Deployment.Up
		fs.Inputs = work.Deployment.Inputs
	}

	if work.Task != nil {
		workRef = workRef.
			SetSubPathType(refs.SubPathTypeTask).
			SetSubPath(work.Task.Name)
		mWork.Type = models.JobTypeTask
		fs.Fn = work.Task.Fn
		fs.Inputs = work.Task.Inputs
	}

	workRefString, err := refstore.IncrementPath(ctx, r.Store, fmt.Sprintf("%s/", workRef.String()))
	if err != nil {
		return refs.Ref{}, models.Run{}, nil, fmt.Errorf("failed to increment path: %w", err)
	}
	workRef, err = refs.Parse(workRefString)
	if err != nil {
		return refs.Ref{}, models.Run{}, nil, fmt.Errorf("failed to parse path: %w", err)
	}

	return workRef, mWork, fs, nil
}

// TODO: This needs updating to use the previous chain
func (r *releaseStore) sdkWorkToFunctionChainDown(
	ctx context.Context,
	environment string,
	fn *models.Function,
	downFunc sdk.FunctionDef,
) (refs.Ref, models.Run, *models.Function, error) {
	workRef := r.ReleaseRef
	workRef = workRef.
		SetSubPathType(refs.SubPathTypeDeploy).
		SetSubPath(environment)

	mWork := models.Run{
		Type:    models.JobTypeDown,
		Release: r.ReleaseRef,
	}
	fs := &models.Function{
		Fn:     downFunc,
		Inputs: make(map[string]sdk.InputDescriptor),
	}

	for name, input := range fn.Inputs {
		i := sdk.InputDescriptor{
			Value: input.Value,
		}
		if input.Value == nil {
			i.Value = input.Default
		}
		fs.Inputs[name] = i
	}

	workRefString, err := refstore.IncrementPath(ctx, r.Store, fmt.Sprintf("%s/", workRef.String()))
	if err != nil {
		return refs.Ref{}, models.Run{}, nil, fmt.Errorf("failed to increment path: %w", err)
	}
	workRef, err = refs.Parse(workRefString)
	if err != nil {
		return refs.Ref{}, models.Run{}, nil, fmt.Errorf("failed to parse path: %w", err)
	}

	return workRef, mWork, fs, nil
}

// SDKPackageToFunctions converts a SDK package to a map of function chain refs to
// function summaries representing the first function in each chain
func (r *releaseStore) SDKPackageToFunctions(ctx context.Context, pkg *sdk.Package) (map[refs.Ref]*models.Function, error) {
	functionChains := make(map[refs.Ref]*models.Function)
	var previousPhaseRefs []refs.Ref
	for _, phase := range pkg.Phases {
		var currentPhaseRefs []refs.Ref
		for _, work := range phase.Tasks {
			workRef, _, fc, err := r.sdkWorkToFunctionChain(ctx, work)
			if err != nil {
				return nil, err
			}
			fc.Dependencies = previousPhaseRefs
			functionChains[workRef] = fc

			// Create a matchable ref for the dependency
			dependencyRef := workRef
			if _, err := strconv.Atoi(path.Base(dependencyRef.SubPath)); err == nil {
				dependencyRef.SubPath = path.Join(path.Dir(dependencyRef.SubPath), "*")
			}
			dependencyRef = dependencyRef.JoinSubPath("status", string(models.StatusComplete))
			currentPhaseRefs = append(currentPhaseRefs, dependencyRef)
		}
		previousPhaseRefs = currentPhaseRefs
	}
	return functionChains, nil
}
