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

	err = r.stateStore.Store.StartTransaction(ctx, "initializing release")
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		commitErr := r.stateStore.Store.CommitTransaction(ctx)
		if commitErr != nil {
			log.Error("failed to commit transaction", "error", commitErr)
		}
	}()

	ref := r.stateStore.ReleaseRef
	// Create all our jobs up front
	jobs, err := r.stateStore.SDKPackageToFunctions(ctx, r.pkg)
	if err != nil {
		return fmt.Errorf("failed to create function chains: %w", err)
	}
	for jobRef, fn := range jobs {
		var t models.JobType
		if jobRef.SubPathType == refs.SubPathTypeCall {
			t = models.JobTypeTask
		} else {
			t = models.JobTypeUp
		}

		err = r.stateStore.InitializeFunction(ctx, models.Run{
			Type:    t,
			Release: r.stateStore.ReleaseRef,
		}, jobRef, fn)
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

// UnfilteredNextWork returns all work that is pending execution,
// regardless of whether or not their inputs are available.
func (r *ReleaseTracker) UnfilteredNextWork(ctx context.Context) (map[refs.Ref]*models.Run, error) {
	nf, err := r.stateStore.PendingJobs(ctx)
	if err != nil {
		return nil, err
	}

	return nf, nil
}

// FilteredNextWork returns all work that is pending execution,
// but only those that have all their inputs available.
func (r *ReleaseTracker) FilteredNextWork(ctx context.Context) (map[refs.Ref]*models.Run, error) {
	if err := r.stateStore.Store.StartTransaction(ctx, "populating inputs"); err != nil {
		return nil, err
	}
	defer func() {
		if err := r.stateStore.Store.CommitTransaction(ctx); err != nil {
			log.Error("failed to commit transaction", "error", err)
		}
	}()

	nw, err := r.UnfilteredNextWork(ctx)
	if err != nil {
		return nil, err
	}

	log.Info("Filtering pending work", "work", nw)

	out := make(map[refs.Ref]*models.Run)

	for wr, work := range nw {
		missing, err := r.PopulateInputs(ctx, wr, work.Functions[len(work.Functions)-1])
		if err != nil {
			log.Error("failed to populate inputs", "function", wr.String(), "error", err)
			return nil, err
		}
		if len(missing) == 0 {
			out[wr] = work
		} else {
			log.Info("function missing inputs", "function", wr.String(), "missing", missing)
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
		nw  map[refs.Ref]*models.Run
		err error
	)
	for nw, err = r.FilteredNextWork(ctx); len(nw) > 0; nw, err = r.FilteredNextWork(ctx) {
		if err != nil {
			return fmt.Errorf("failed to get next functions: %w", err)
		}

		for workRef, work := range nw {
			chainName := path.Join(string(workRef.SubPathType), strings.Split(workRef.SubPath, "/")[0])

			if _, ok := chainSpan[chainName]; !ok {
				chainCtx[chainName], chainSpan[chainName] = tracer.Start(ctx, fmt.Sprintf("CHAIN %s", chainName))
			}

			var result sdk.WorkResult
			result, err = r.Run(chainCtx[chainName], func(log sdk.Log) {
				logger(workRef, log)
			}, workRef, work)
			if err != nil {
				log.Error("failed to run function", "work", workRef.String(), "error", err)
				return fmt.Errorf("failed to run function %s: %w", workRef.String(), err)
			}

			if result.Err != nil {
				logger(workRef, sdk.Log{
					Timestamp: time.Now(),
					Message:   result.Err.Error(),
				})
			}

			log.Info("work done", "work", workRef.String())

			// Check if the chain or phase is now complete
			chainStatus, err := r.stateStore.GetWorkStatus(ctx, workRef)
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
	failedJobs, err := r.stateStore.FailedJobs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get failed functions: %w", err)
	}

	if len(failedJobs) == 0 {
		return errors.New("no failed functions found")
	}

	for jobRef, job := range failedJobs {
		log.Info("retrying function", "ref", jobRef.String(), "function", job)

		// Create an incremented chain ref for the retry
		chainRef := ReduceToJobRef(jobRef)
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

		// Update the parent chain status to failed_retried as well
		originalChainRef := ReduceToJobRef(jobRef)
		if err := saveStatus(ctx, r.stateStore.Store, originalChainRef, models.StatusFailedRetried); err != nil {
			return fmt.Errorf("failed to update original chain status: %w", err)
		}

		fn := job.Functions[0]

		// Reset the function state for retry - create a clean copy
		retryFn := models.Function{
			Fn:           fn.Fn,
			Dependencies: fn.Dependencies,
			Inputs:       fn.Inputs,
		}

		// Initialize the work state for the new chain using the standalone function
		if err := InitializeRun(ctx, r.stateStore.Store, incrementedChainRef, &retryFn); err != nil {
			return fmt.Errorf("failed to initialize retry chain: %w", err)
		}

		// The retry function is ready to execute since inputs were already populated
		// in the previous failed run. It will be picked up by the normal execution flow.
		log.Info("retry function created and ready for execution",
			"ref", incrementedChainRef.String(),
			"inputs_available", len(retryFn.Inputs) > 0)
	}

	return r.RunToPause(ctx, logger)
}

func (r *ReleaseTracker) Task(ctx context.Context, ref string, logger Logger) error {
	err := r.stateStore.Store.StartTransaction(ctx, "running task")
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		commitErr := r.stateStore.Store.CommitTransaction(ctx)
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
	functionChains, err := r.stateStore.SDKPackageToFunctions(ctx, r.pkg)
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

		err = r.stateStore.InitializeFunction(ctx, models.Run{
			Type:    models.JobTypeUp,
			Release: r.stateStore.ReleaseRef,
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
	log.Info("Populating inputs", "ref", fnRef.String())
	if fn.Inputs == nil {
		fn.Inputs = make(map[string]sdk.InputDescriptor)
	}

	// Add chain inputs to function if this is the first function
	inputs, err := PopulateInputs(ctx, r.stateStore.Store, fn.Inputs)
	if err != nil {
		return nil, err
	}
	fn.Inputs = inputs

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
			log.Info("input not found", "ref", r.String())
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

func (r *ReleaseTracker) updateIntent(ctx context.Context, workRef refs.Ref, work *models.Run) error {
	// We only want intents for deployments
	if workRef.SubPathType != refs.SubPathTypeDeploy {
		return nil
	}
	// Only update the first time we run a deployment
	if len(work.Functions) != 1 {
		return nil
	}

	fn := work.Functions[0]

	workIntent := workRef.MakeIntent().SetVersion("")
	workIntent = workIntent.SetSubPath(path.Dir(workIntent.SubPath))
	intent := models.Intent{
		Release: r.stateStore.ReleaseRef,
		Inputs:  fn.Inputs,
	}

	if err := r.stateStore.Store.Set(ctx, workIntent.String(), intent); err != nil {
		return fmt.Errorf("failed to set intent state: %w", err)
	}

	return nil
}

func (r *ReleaseTracker) Run(
	ctx context.Context,
	logger sdk.Logger,
	workRef refs.Ref,
	work *models.Run,
) (sdk.WorkResult, error) {
	log.Info("Run", "work", workRef.String())
	action := fmt.Sprintf("WORK %s", workRef.String())
	ctx, span := tracer.Start(ctx, action)
	defer span.End()

	workName := strings.Split(workRef.SubPath, "/")[0]

	span.SetAttributes(
		attribute.String(AttributeCICDPipelineTaskName, workName),
		attribute.String(AttributeCICDPipelineTaskRunType, "run"),
	)

	log.Info("running function", "function", workRef.String())

	if err := r.stateStore.Store.StartTransaction(ctx, "work started\n\n"+workRef.String()); err != nil {
		return sdk.WorkResult{}, fmt.Errorf("failed to start transaction: %w", err)
	}

	// Set status of work
	if err := saveStatus(ctx, r.stateStore.Store, workRef, models.StatusRunning); err != nil {
		return sdk.WorkResult{}, fmt.Errorf("failed to save status: %w", err)
	}

	// Set intent if appropriate
	if err := r.updateIntent(ctx, workRef, work); err != nil {
		return sdk.WorkResult{}, fmt.Errorf("failed to update intent state: %w", err)
	}
	if err := r.stateStore.Store.CommitTransaction(ctx); err != nil {
		log.Error("failed to commit transaction", "error", err)
	}

	if err := r.stateStore.Store.StartTransaction(ctx, "work finished\n\n"+workRef.String()); err != nil {
		return sdk.WorkResult{}, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err := r.stateStore.Store.CommitTransaction(ctx); err != nil {
			log.Error("failed to commit transaction", "error", err)
		}
	}()

	// Start from the final function in the chain
	fn := work.Functions[len(work.Functions)-1]

	for fn != nil {
		fnCtx := sdk.FunctionContext{
			Inputs: make(map[string]any),
		}
		for k, v := range fn.Inputs {
			if v.Value == nil {
				fnCtx.Inputs[k] = v.Default
			} else {
				fnCtx.Inputs[k] = v.Value
			}
		}

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

		// If we completed but no result was provided, assume success.
		// This handles when someone forgot an empty done().
		if result.Err == nil && result.Done == nil && result.Next == nil {
			result.Done = &sdk.WorkDone{}
		}

		// Record the result of this function to the state store
		if err := r.saveWorkState(ctx, workRef, work, result, logs); err != nil {
			return result, fmt.Errorf("failed to save work state: %w", err)
		}

		if result.Err != nil {
			log.Error("function failed", "function", fn.Fn.Name, "error", result.Err)
			span.SetAttributes(
				attribute.String(AttributeCICDPipelineTaskRunResult, "failure"),
				attribute.String(AttributeErrorType, "fail"),
			)
			return result, nil
		}

		// All other results mean this function succeeded
		span.SetAttributes(
			attribute.String(AttributeCICDPipelineTaskRunResult, "success"),
		)

		if result.Done != nil {
			work.Outputs = result.Done.Outputs
			return result, nil
		}

		if result.Next != nil {
			nextFunction := &models.Function{
				Fn:     result.Next.Fn,
				Inputs: result.Next.Inputs,
			}
			if err := validateFunction(nextFunction); err != nil {
				return sdk.WorkResult{}, fmt.Errorf("failed to validate function: %w", err)
			}

			nextFunction.Inputs, err = PopulateInputs(ctx, r.stateStore.Store, nextFunction.Inputs)
			if err != nil {
				return sdk.WorkResult{}, fmt.Errorf("failed to populate inputs for %s: %w", nextFunction.Fn.Name, err)
			}
			work.Functions = append(work.Functions, nextFunction)

			missing := GetMissing(nextFunction.Inputs)
			if len(missing) > 0 {
				log.Info("Next function was missing inputs", "missing", missing)
				// Update state to ensure we capture the next function
				if err := r.saveWorkState(ctx, workRef, work, result, logs); err != nil {
					return result, fmt.Errorf("failed to save work state: %w", err)
				}
				return result, nil
			}

			fn = nextFunction
		}
	}
	return sdk.WorkResult{}, errors.New("next or done was not called")
}

func ResultToStatus(result sdk.WorkResult) models.Status {
	if result.Next != nil {
		return models.StatusPaused
	}
	if result.Err != nil {
		return models.StatusFailed
	}
	if result.Done != nil {
		return models.StatusComplete
	}
	return models.StatusRunning
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

func (r *ReleaseTracker) saveWorkState(ctx context.Context, runRef refs.Ref, run *models.Run, result sdk.WorkResult, logs []sdk.Log) error {
	// Append logs
	if err := r.updateLogs(ctx, runRef, logs); err != nil {
		return err
	}

	status := ResultToStatus(result)
	log.Info("Setting status", "ref", runRef.String(), "status", status)
	if err := saveStatus(ctx, r.stateStore.Store, runRef, status); err != nil {
		return fmt.Errorf("failed to save work status: %w", err)
	}

	if result.Done != nil {
		run.Outputs = result.Done.Outputs
	}

	if err := r.stateStore.Store.Set(ctx, runRef.String(), run); err != nil {
		return fmt.Errorf("failed to set work: %w", err)
	}

	// If the work completed successfully, record it as the most recent work ref
	if status != models.StatusComplete {
		return nil
	}

	if err := r.stateStore.AddTags(ctx, result.Done.Tags); err != nil {
		return fmt.Errorf("failed to add tags: %w", err)
	}

	log.Info("Setting run", "ref", runRef.String())
	if err := r.stateStore.Store.Set(ctx, runRef.String(), run); err != nil {
		return fmt.Errorf("failed to set work: %w", err)
	}
	taskRef, err := refs.Reduce(runRef.String(), GlobWork)
	if err != nil {
		return fmt.Errorf("failed to reduce work ref: %w", err)
	}
	log.Info("Setting task", "ref", taskRef)
	task := models.Task{
		RunRef: runRef,
		Intent: models.Intent{
			Type:    run.Type,
			Release: run.Release,
		},
		Outputs: run.Outputs,
	}
	if len(run.Functions) > 0 {
		task.Inputs = run.Functions[0].Inputs
	}
	if err := r.stateStore.Store.Set(ctx, taskRef, task); err != nil {
		return fmt.Errorf("failed to link work: %w", err)
	}

	taskRefParsed, err := refs.Parse(taskRef)
	if err != nil {
		return fmt.Errorf("failed to parse task ref: %w", err)
	}
	latestReleaseTaskRef := taskRefParsed.MakeRelease().SetVersion("")
	if run.Type == models.JobTypeDown {
		if err := r.stateStore.Store.Unlink(ctx, latestReleaseTaskRef.String()); err != nil {
			return fmt.Errorf("failed to unlink work: %w", err)
		}
	} else {
		if err := r.stateStore.Store.Link(ctx, latestReleaseTaskRef.String(), taskRef); err != nil {
			return fmt.Errorf("failed to link work: %w", err)
		}
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
