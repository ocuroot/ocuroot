package release

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"
	"github.com/ocuroot/ocuroot/ui/components/pipeline"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func NewReleaseTracker(ctx context.Context, config *sdk.Config, pkg *sdk.Package, releaseRef refs.Ref, store refstore.Store) (*ReleaseTracker, error) {
	resolvedReleaseRefString, err := store.ResolveLink(ctx, releaseRef.String())
	if err != nil {
		return nil, err
	}

	resolvedReleaseRef, err := refs.Parse(resolvedReleaseRefString)
	if err != nil {
		return nil, err
	}

	stateStore, err := ReleaseStore(ctx, resolvedReleaseRef.String(), store)
	if err != nil {
		return nil, err
	}

	return &ReleaseTracker{
		config:     config,
		pkg:        pkg,
		stateStore: stateStore,
		ReleaseRef: resolvedReleaseRef,
	}, nil
}

type ReleaseTracker struct {
	config     *sdk.Config
	pkg        *sdk.Package
	stateStore *releaseStore
	ReleaseRef refs.Ref
}

func (r *ReleaseTracker) ReleaseStatus(ctx context.Context) (models.Status, error) {
	statuses, err := r.stateStore.Store.Match(ctx, r.stateStore.ReleaseRef.String()+"/**/status/*")
	if err != nil {
		return "", fmt.Errorf("failed to match release status: %w", err)
	}

	var allStatuses []models.Status

	for _, statusRef := range statuses {
		status := models.Status(path.Base(statusRef))
		allStatuses = append(allStatuses, status)
	}

	isPending := true
	for _, status := range allStatuses {
		if status != models.StatusPending {
			isPending = false
			break
		}
	}

	if isPending {
		return models.StatusPending, nil
	}

	var out models.Status = models.StatusComplete
	for _, status := range allStatuses {
		if status == models.StatusFailed {
			return models.StatusFailed, nil
		}
		if status == models.StatusRunning {
			return models.StatusRunning, nil
		}
		if status == models.StatusCancelled {
			return models.StatusCancelled, nil
		}
	}

	return out, nil
}

type ReleaseInfo struct {
	Commit  string       `json:"commit"`
	Package *sdk.Package `json:"package"`
}

func (r *ReleaseTracker) GetReleaseInfo(ctx context.Context) (*ReleaseInfo, error) {
	return r.stateStore.GetReleaseInfo(ctx)
}

func (r *ReleaseTracker) InitRelease(ctx context.Context, commit string) error {
	var err error

	err = r.stateStore.Store.StartTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		commitErr := r.stateStore.Store.CommitTransaction(ctx, "initializing release")
		if commitErr != nil {
			log.Error("failed to commit transaction", "error", commitErr)
		}
	}()

	ref := r.stateStore.ReleaseRef
	// Create all our function chains up front
	functionChains, err := r.stateStore.SDKPackageToFunctionChains(ctx, r.pkg)
	if err != nil {
		return fmt.Errorf("failed to create function chains: %w", err)
	}
	for functionChainRef, fn := range functionChains {
		functionRef := FunctionRefFromChainRef(functionChainRef, fn)
		err = r.stateStore.InitializeFunction(ctx, models.Work{
			Release:    r.stateStore.ReleaseRef,
			Entrypoint: functionRef,
		}, functionChainRef, fn)
		if err != nil {
			return fmt.Errorf("failed to initialize function: %w", err)
		}
	}

	releaseInfo := ReleaseInfo{
		Commit:  commit,
		Package: r.pkg,
	}

	if err := r.stateStore.Store.Set(
		ctx,
		ref.String(),
		releaseInfo,
	); err != nil {
		return fmt.Errorf("failed to set release state: %w", err)
	}

	// Set commit marker
	if err := r.stateStore.Store.Set(
		ctx,
		ref.SetSubPathType(refs.SubPathTypeCommit).SetSubPath(commit).String(),
		models.NewMarker(),
	); err != nil {
		return fmt.Errorf("failed to set release state: %w", err)
	}

	return nil
}

func (r *ReleaseTracker) GetReleaseSummary(ctx context.Context) (*pipeline.ReleaseSummary, error) {
	return r.stateStore.GetReleaseState(ctx)
}

// UnfilteredNextFunctions returns all functions that are pending execution,
// regardless of whether or not their inputs are available.
func (r *ReleaseTracker) UnfilteredNextFunctions(ctx context.Context) (map[refs.Ref]*models.Function, error) {
	nf, err := r.stateStore.PendingFunctions(ctx)
	if err != nil {
		return nil, err
	}

	return nf, nil
}

// FilteredNextFunctions returns all functions that are pending execution,
// but only those that have all their inputs available.
func (r *ReleaseTracker) FilteredNextFunctions(ctx context.Context) (map[refs.Ref]*models.Function, error) {
	if err := r.stateStore.Store.StartTransaction(ctx); err != nil {
		return nil, err
	}
	defer func() {
		if err := r.stateStore.Store.CommitTransaction(ctx, "populating inputs"); err != nil {
			log.Error("failed to commit transaction", "error", err)
		}
	}()

	nf, err := r.UnfilteredNextFunctions(ctx)
	if err != nil {
		return nil, err
	}

	log.Info("Filtering pending functions", "functions", nf)

	out := make(map[refs.Ref]*models.Function)

	for fr, fn := range nf {
		missing, err := r.PopulateInputs(ctx, fr, fn)
		if err != nil {
			log.Error("failed to populate inputs", "function", fr.String(), "error", err)
			return nil, err
		}
		if len(missing) == 0 {
			out[fr] = fn
		} else {
			log.Info("function missing inputs", "function", fr.String(), "missing", missing)
		}
	}

	log.Info("Filtered functions", "count", len(out))

	return out, nil
}

func (r *ReleaseTracker) RunToPause(ctx context.Context, logger Logger) error {
	log.Info("RunToPause", "release", r.stateStore.ReleaseRef.String())
	pipelineName := refs.Ref{
		Repo:     r.stateStore.ReleaseRef.Repo,
		Filename: r.stateStore.ReleaseRef.Filename,
	}

	ctx, span := tracer.Start(ctx, fmt.Sprintf("RUN %s", pipelineName.String()))
	defer span.End()

	span.SetAttributes(
		attribute.String(AttributeCICDPipelineName, pipelineName.String()),
		attribute.String(AttributeCICDPipelineActionName, "run"),
		attribute.String(AttributeCICDPipelineRunID, r.stateStore.ReleaseRef.String()),
	)

	var (
		chainCtx  map[string]context.Context = make(map[string]context.Context)
		chainSpan map[string]trace.Span      = make(map[string]trace.Span)
	)

	var (
		fns map[refs.Ref]*models.Function
		err error
	)
	for fns, err = r.FilteredNextFunctions(ctx); len(fns) > 0; fns, err = r.FilteredNextFunctions(ctx) {
		if err != nil {
			return fmt.Errorf("failed to get next functions: %w", err)
		}

		for fr, fn := range fns {
			chainRef := ChainRefFromFunctionRef(fr)
			chainName := path.Join(string(chainRef.SubPathType), strings.Split(chainRef.SubPath, "/")[0])

			if _, ok := chainSpan[chainName]; !ok {
				chainCtx[chainName], chainSpan[chainName] = tracer.Start(ctx, fmt.Sprintf("CHAIN %s", chainName))
			}

			var result sdk.WorkResult
			result, err = r.Run(chainCtx[chainName], func(log sdk.Log) {
				logger(fr, log)
			}, fr, fn)
			if err != nil {
				return fmt.Errorf("failed to run function %s: %w", fn.Fn, err)
			}

			if result.Err != nil {
				logger(fr, sdk.Log{
					Timestamp: time.Now(),
					Message:   result.Err.Error(),
				})
			}

			log.Info("function done", "function", fn.Fn.Name)

			// Check if the chain or phase is now complete
			chainStatus, err := r.stateStore.GetFunctionChainStatus(ctx, chainRef)
			if err != nil {
				return fmt.Errorf("failed to get function chain status: %w", err)
			}
			if chainStatus != models.StatusRunning && chainStatus != models.StatusPending {
				chainSpan[chainName].End()
			}
		}
	}
	if err != nil {
		return fmt.Errorf("failed to get next functions: %w", err)
	}

	releaseStatus, err := r.ReleaseStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get release state: %w", err)
	}

	span.SetAttributes(
		// TODO: Map this to success;failure;timeout;skipped
		attribute.String(AttributeCICDPipelineRunState, string(releaseStatus)),
	)

	return nil
}

func (r *ReleaseTracker) Retry(ctx context.Context, logger Logger) error {
	failedFunctions, err := r.stateStore.FailedFunctions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get failed functions: %w", err)
	}

	if len(failedFunctions) == 0 {
		return errors.New("no failed functions found")
	}

	for fnRef, fn := range failedFunctions {
		log.Info("retrying function", "ref", fnRef.String(), "function", fn)

		// Create an incremented chain ref for the retry
		chainRef := ChainRefFromFunctionRef(fnRef)
		chainRefStr := chainRef.String()
		lastSlashIndex := strings.LastIndex(chainRefStr, "/")
		if lastSlashIndex == -1 {
			return fmt.Errorf("invalid chain ref format: %s", chainRefStr)
		}
		chainRefPrefix := chainRefStr[:lastSlashIndex+1] // includes the trailing slash

		incrementedChainPath, err := refstore.IncrementPath(ctx, r.stateStore.Store, chainRefPrefix)
		if err != nil {
			return fmt.Errorf("failed to increment chain ref: %w", err)
		}

		incrementedChainRef, err := refs.Parse(incrementedChainPath)
		if err != nil {
			return fmt.Errorf("failed to parse incremented chain ref: %w", err)
		}

		// Create the function ref within the new incremented chain
		incrementedFnRef := incrementedChainRef.JoinSubPath("functions").JoinSubPath(string(fn.ID))

		// Update the original failed function to indicate it has been retried
		originalFn := *fn
		originalFn.Status = models.StatusFailedRetried
		if err := UpdateFunctionStateUnderRef(ctx, r.stateStore.Store, fnRef, &originalFn); err != nil {
			return fmt.Errorf("failed to update original function status: %w", err)
		}

		// Update the parent chain status to failed_retried as well
		originalChainRef := ChainRefFromFunctionRef(fnRef)
		if err := saveStatus(ctx, r.stateStore.Store, originalChainRef, models.StatusFailedRetried); err != nil {
			return fmt.Errorf("failed to update original chain status: %w", err)
		}

		// Reset the function state for retry - create a clean copy
		retryFn := models.Function{
			ID:           fn.ID,
			Fn:           fn.Fn,
			Status:       models.StatusPending,
			Dependencies: fn.Dependencies,
			Inputs:       fn.Inputs,
			Outputs:      nil, // Clear any previous outputs
		}

		// Store the function at the incremented ref
		if err := UpdateFunctionStateUnderRef(ctx, r.stateStore.Store, incrementedFnRef, &retryFn); err != nil {
			return fmt.Errorf("failed to store retry function: %w", err)
		}

		// Initialize the work state for the new chain using the standalone function
		if err := InitializeFunctionChain(ctx, r.stateStore.Store, incrementedChainRef, &retryFn); err != nil {
			return fmt.Errorf("failed to initialize retry chain: %w", err)
		}

		// The retry function is ready to execute since inputs were already populated
		// in the previous failed run. It will be picked up by the normal execution flow.
		log.Info("retry function created and ready for execution",
			"ref", incrementedFnRef.String(),
			"inputs_available", len(retryFn.Inputs) > 0)
	}

	return r.RunToPause(ctx, logger)
}

func (r *ReleaseTracker) Task(ctx context.Context, ref string, logger Logger) error {
	err := r.stateStore.Store.StartTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		commitErr := r.stateStore.Store.CommitTransaction(ctx, "running task")
		if commitErr != nil {
			log.Error("failed to commit transaction", "error", commitErr)
		}
	}()

	pr, err := refs.Parse(ref)
	if err != nil {
		return err
	}

	if pr.SubPath == "check_envs" {
		err = r.checkEnvs(ctx, logger)
		if err != nil {
			return fmt.Errorf("failed to check environments: %w", err)
		}
	}

	// Remove the task now it is complete
	return r.stateStore.Store.Delete(ctx, ref)
}

func (r *ReleaseTracker) checkEnvs(ctx context.Context, logger Logger) error {
	var err error

	ref := r.stateStore.ReleaseRef
	// Create all our function chains up front
	functionChains, err := r.stateStore.SDKPackageToFunctionChains(ctx, r.pkg)
	if err != nil {
		return fmt.Errorf("failed to create function chains: %w", err)
	}

	for functionChainRef, fn := range functionChains {
		// Only create a chain for the first run
		// This will capture new environments
		if path.Base(functionChainRef.String()) != "1" {
			continue
		}

		log.Info("Creating chain", "ref", functionChainRef.String())

		functionRef := FunctionRefFromChainRef(functionChainRef, fn)
		err = r.stateStore.InitializeFunction(ctx, models.Work{
			Release:    r.stateStore.ReleaseRef,
			Entrypoint: functionRef,
		}, functionChainRef, fn)
		if err != nil {
			return fmt.Errorf("failed to initialize function: %w", err)
		}
	}

	// Update the package info
	var releaseInfo ReleaseInfo
	err = r.stateStore.Store.Get(ctx, ref.String(), &releaseInfo)
	if err != nil {
		return fmt.Errorf("failed to get release state: %w", err)
	}

	releaseInfo.Package = r.pkg

	if err := r.stateStore.Store.Set(
		ctx,
		ref.String(),
		releaseInfo,
	); err != nil {
		return fmt.Errorf("failed to set release state: %w", err)
	}

	return nil
}

func (r *ReleaseTracker) PopulateInputs(ctx context.Context, fnRef refs.Ref, fn *models.Function) ([]sdk.InputDescriptor, error) {
	if fn.Inputs == nil {
		fn.Inputs = make(map[string]sdk.InputDescriptor)
	}

	// Add chain inputs to function if this is the first function
	inputs, err := PopulateInputs(ctx, r.stateStore.Store, fn.Inputs)
	if err != nil {
		return nil, err
	}
	fn.Inputs = inputs

	if err := UpdateFunctionStateUnderRef(ctx, r.stateStore.Store, fnRef, fn); err != nil {
		return nil, fmt.Errorf("failed to update function state: %w", err)
	}

	missing := GetMissing(fn.Inputs)
	return missing, nil
}

// RetrieveInput implements ReleaseStateStore.
func RetrieveInput(ctx context.Context, store refstore.Store, input sdk.InputDescriptor) (sdk.InputDescriptor, error) {
	if input.Ref == nil {
		if input.Value == nil {
			return sdk.InputDescriptor{}, fmt.Errorf("either a static value or a ref must be provided")
		}
		return input, nil
	}

	r := *input.Ref
	var newValue any
	err := store.Get(ctx, r.String(), &newValue)
	if err != nil {
		if errors.Is(err, refstore.ErrRefNotFound) {
			return input, nil
		}
		return input, err
	}

	input.Value = newValue

	return input, nil
}

func GetMissing(inputs map[string]sdk.InputDescriptor) []sdk.InputDescriptor {
	var missing []sdk.InputDescriptor
	for _, d := range inputs {
		if d.Value == nil && d.Default == nil {
			missing = append(missing, d)
		}
	}
	return missing
}

func PopulateInputs(ctx context.Context, store refstore.Store, inputs map[string]sdk.InputDescriptor) (map[string]sdk.InputDescriptor, error) {
	out := make(map[string]sdk.InputDescriptor)
	for k, d := range inputs {
		v, err := RetrieveInput(ctx, store, d)
		if err != nil && !errors.Is(err, refstore.ErrRefNotFound) {
			return nil, fmt.Errorf("failed to retrieve input %s: %w", k, err)
		}
		out[k] = v
	}
	return out, nil
}

func (r *ReleaseTracker) updateIntent(ctx context.Context, fnRef refs.Ref, fn *models.Function) error {
	// We only want intents for deployments
	if fnRef.SubPathType != refs.SubPathTypeDeploy {
		return nil
	}

	chainRef := ChainRefFromFunctionRef(fnRef)

	var work models.Work
	if err := r.stateStore.Store.Get(ctx, chainRef.String(), &work); err != nil {
		return fmt.Errorf("failed to get work state: %w", err)
	}

	log.Info("Checking intent", "entrypoint", work.Entrypoint.String(), "fn", fnRef.String())

	if work.Entrypoint.String() != fnRef.String() {
		log.Info("Not an intent", "entrypoint", work.Entrypoint.String(), "fn", fnRef.String())
		return nil
	}

	chainIntent, err := WorkRefFromChainRef(chainRef)
	if err != nil {
		return fmt.Errorf("failed to make intent: %w", err)
	}
	chainIntent = chainIntent.MakeIntent().SetVersion("")
	intent := models.Intent{
		Release: r.stateStore.ReleaseRef,
		Inputs:  fn.Inputs,
	}

	if err := r.stateStore.Store.Set(ctx, chainIntent.String(), intent); err != nil {
		return fmt.Errorf("failed to set intent state: %w", err)
	}

	return nil
}

func (r *ReleaseTracker) Run(
	ctx context.Context,
	logger sdk.Logger,
	fnRef refs.Ref,
	fn *models.Function,
) (sdk.WorkResult, error) {
	log.Info("Run", "function", fnRef.String())
	action := fmt.Sprintf("FUNCTION %s", fnRef.String())
	ctx, span := tracer.Start(ctx, action)
	defer span.End()

	span.SetAttributes(
		attribute.String(AttributeCICDPipelineTaskName, string(fn.Fn.Name)),
		attribute.String(AttributeCICDPipelineTaskRunID, string(fn.ID)),
		attribute.String(AttributeCICDPipelineTaskRunType, "run"),
	)

	log.Info("running function", "function", fn.Fn.Name)

	if err := r.stateStore.Store.StartTransaction(ctx); err != nil {
		return sdk.WorkResult{}, fmt.Errorf("failed to start transaction: %w", err)
	}

	fn.Status = models.StatusRunning
	if err := UpdateFunctionStateUnderRef(ctx, r.stateStore.Store, fnRef, fn); err != nil {
		return sdk.WorkResult{}, fmt.Errorf("failed to update function state: %w", err)
	}
	// Set intent if appropriate
	if err := r.updateIntent(ctx, fnRef, fn); err != nil {
		return sdk.WorkResult{}, fmt.Errorf("failed to update intent state: %w", err)
	}
	if err := r.stateStore.Store.CommitTransaction(ctx, "function started\n\n"+fnRef.String()); err != nil {
		log.Error("failed to commit transaction", "error", err)
	}

	fnCtx := sdk.FunctionContext{
		WorkID: sdk.WorkID(fn.ID),
		Inputs: make(map[string]any),
	}
	for k, v := range fn.Inputs {
		if v.Value == nil {
			fnCtx.Inputs[k] = v.Default
		} else {
			fnCtx.Inputs[k] = v.Value
		}
	}

	if err := r.stateStore.Store.StartTransaction(ctx); err != nil {
		return sdk.WorkResult{}, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err := r.stateStore.Store.CommitTransaction(ctx, "function finished\n\n"+fnRef.String()); err != nil {
			log.Error("failed to commit transaction", "error", err)
		}
	}()

	var logs []sdk.Log
	innerLogger := func(log sdk.Log) {
		logs = append(logs, log)
		logger(log)
	}

	result, err := r.config.Run(
		ctx,
		fn.Fn,
		innerLogger,
		fnCtx,
	)
	if err != nil {
		return sdk.WorkResult{}, fmt.Errorf("failed to run function %s: %w", fn.Fn, err)
	}

	fn.Status = models.StatusComplete

	if result.Err != nil {
		log.Error("function failed", "function", fn.Fn.Name, "error", result.Err)
		fn.Status = models.StatusFailed

		span.SetAttributes(
			attribute.String(AttributeCICDPipelineTaskRunResult, "failure"),
			attribute.String(AttributeErrorType, "fail"),
		)
	} else {
		// All other results mean this function succeeded
		span.SetAttributes(
			attribute.String(AttributeCICDPipelineTaskRunResult, "success"),
		)
	}

	if result.Done != nil {
		fn.Outputs = result.Done.Outputs
	}

	chainRef := ChainRefFromFunctionRef(fnRef)

	// Record the result of this function to the state store
	if err := r.saveWorkState(ctx, chainRef, result, logs); err != nil {
		return result, fmt.Errorf("failed to save work state: %w", err)
	}

	var nextFunction *models.Function

	if result.Next != nil {
		idNum, err := strconv.Atoi(string(fn.ID))
		if err != nil {
			return result, fmt.Errorf("failed to parse function ID: %w", err)
		}
		idNum++

		nextFunction = &models.Function{
			ID:     models.FunctionID(fmt.Sprint(idNum)),
			Fn:     result.Next.Fn,
			Status: models.StatusPending,
			Inputs: result.Next.Inputs,
		}

		// Record the next function
		log.Info("Recording pending function", "id", nextFunction.ID, "fn", nextFunction.Fn)
		nextFnRef := fnRef.SetSubPath(strings.Split(fnRef.SubPath, "/functions/")[0] + "/functions/" + string(nextFunction.ID))
		if err := UpdateFunctionStateUnderRef(ctx, r.stateStore.Store, nextFnRef, nextFunction); err != nil {
			log.Error("failed to update function state", "error", err)
			fn.Status = models.StatusFailed
			logger(sdk.Log{
				Timestamp: time.Now(),
				Message:   err.Error(),
			})
		}
	}

	if err := UpdateFunctionStateUnderRef(ctx, r.stateStore.Store, fnRef, fn); err != nil {
		log.Error("failed to set function state", "error", err)
	}

	return result, nil
}

func ResultToStatus(result sdk.WorkResult) models.Status {
	if result.Next != nil {
		return models.StatusRunning
	}
	if result.Err != nil {
		return models.StatusFailed
	}
	return models.StatusComplete
}

func (r *ReleaseTracker) updateLogs(ctx context.Context, chainRef refs.Ref, logs []sdk.Log) error {
	logRef := chainRef.JoinSubPath("logs")

	// Append logs
	var existingLogs []sdk.Log
	if err := r.stateStore.Store.Get(ctx, logRef.String(), &existingLogs); err != nil && !errors.Is(err, refstore.ErrRefNotFound) {
		return fmt.Errorf("failed to get logs: %w", err)
	}
	existingLogs = append(existingLogs, logs...)
	if err := r.stateStore.Store.Set(ctx, logRef.String(), existingLogs); err != nil {
		return fmt.Errorf("failed to set logs: %w", err)
	}
	return nil
}

func (r *ReleaseTracker) saveWorkState(ctx context.Context, chainRef refs.Ref, result sdk.WorkResult, logs []sdk.Log) error {
	currentWorkRef := chainRef.SetSubPath(path.Dir(chainRef.SubPath))

	// Append logs
	if err := r.updateLogs(ctx, chainRef, logs); err != nil {
		return err
	}

	// Only save other state if complete
	if result.Done == nil && result.Err == nil {
		return nil
	}

	status := ResultToStatus(result)
	if err := saveStatus(ctx, r.stateStore.Store, currentWorkRef, status); err != nil {
		return fmt.Errorf("failed to save work status: %w", err)
	}
	if err := saveStatus(ctx, r.stateStore.Store, chainRef, status); err != nil {
		return fmt.Errorf("failed to save work status: %w", err)
	}

	var workState models.Work
	if err := r.stateStore.Store.Get(ctx, chainRef.String(), &workState); err != nil {
		return fmt.Errorf("failed to get work: %w", err)
	}

	if result.Done != nil {
		workState.Outputs = result.Done.Outputs
	}

	if err := r.stateStore.Store.Set(ctx, chainRef.String(), workState); err != nil {
		return fmt.Errorf("failed to set work: %w", err)
	}

	// If the work completed successfully, record it as the most recent work ref
	if status != models.StatusComplete {
		return nil
	}

	if err := r.stateStore.AddTags(ctx, result.Done.Tags); err != nil {
		return fmt.Errorf("failed to add tags: %w", err)
	}

	if err := r.stateStore.Store.Set(ctx, currentWorkRef.String(), workState); err != nil {
		return fmt.Errorf("failed to set work: %w", err)
	}
	globalCurrentWorkRef := currentWorkRef.MakeRelease().SetVersion("")
	if workState.Type == models.WorkTypeDown {
		if err := r.stateStore.Store.Unlink(ctx, globalCurrentWorkRef.String()); err != nil {
			return fmt.Errorf("failed to unlink work: %w", err)
		}
	} else {
		if err := r.stateStore.Store.Link(ctx, globalCurrentWorkRef.String(), chainRef.String()); err != nil {
			return fmt.Errorf("failed to link work: %w", err)
		}
	}
	// TODO: This will need to be incremented effectively
	absoluteGlobalCurrentWorkRef := chainRef.MakeRelease().SetVersion("")
	if err := r.stateStore.Store.Link(ctx, absoluteGlobalCurrentWorkRef.String(), chainRef.String()); err != nil {
		return fmt.Errorf("failed to link work: %w", err)
	}

	return nil
}

func (r *ReleaseTracker) GetTags(ctx context.Context) ([]string, error) {
	target := r.stateStore.ReleaseRef.String()
	potentialTagRefs, err := r.stateStore.Store.GetLinks(ctx, target)
	if err != nil {
		return nil, err
	}

	tags := make([]string, 0, len(potentialTagRefs))
	for _, tagRef := range potentialTagRefs {
		tp, err := refs.Parse(tagRef)
		if err != nil {
			return nil, err
		}
		shouldMatch := tp.SetVersion(r.ReleaseRef.ReleaseOrIntent.Value)
		if shouldMatch.String() != r.ReleaseRef.String() {
			continue
		}
		tags = append(tags, tp.ReleaseOrIntent.Value)
	}
	return tags, nil
}

func (r *ReleaseTracker) RunDown(ctx context.Context) error {
	return nil
}
