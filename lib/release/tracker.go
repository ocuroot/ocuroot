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

func NewReleaseTracker(
	ctx context.Context,
	config *sdk.Config,
	pkg *sdk.Package,
	releaseRef refs.Ref,
	intent refstore.Store,
	state refstore.Store,
) (*ReleaseTracker, error) {
	resolvedReleaseRefString, err := state.ResolveLink(ctx, releaseRef.String())
	if err != nil {
		return nil, err
	}

	resolvedReleaseRef, err := refs.Parse(resolvedReleaseRefString)
	if err != nil {
		return nil, err
	}

	stateStore, err := ReleaseStore(ctx, resolvedReleaseRef.String(), state)
	if err != nil {
		return nil, err
	}

	return &ReleaseTracker{
		config:     config,
		pkg:        pkg,
		intent:     intent,
		stateStore: stateStore,
		ReleaseRef: resolvedReleaseRef,
	}, nil
}

type ReleaseTracker struct {
	config     *sdk.Config
	pkg        *sdk.Package
	intent     refstore.Store
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
		return fmt.Errorf("failed to create runs: %w", err)
	}
	for jobRef, fn := range jobs {
		var t models.RunType
		if jobRef.SubPathType == refs.SubPathTypeTask {
			t = models.RunTypeTask
		} else {
			t = models.RunTypeUp
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

// UnfilteredNextRun returns all runs that are pending execution,
// regardless of whether or not their inputs are available.
func (r *ReleaseTracker) UnfilteredNextRun(ctx context.Context) (map[refs.Ref]*models.Run, error) {
	nf, err := r.stateStore.PendingJobs(ctx)
	if err != nil {
		return nil, err
	}

	return nf, nil
}

// FilteredNextRun returns any runs that are pending execution,
// but only those that have all their inputs available.
func (r *ReleaseTracker) FilteredNextRun(ctx context.Context) (map[refs.Ref]*models.Run, error) {
	if err := r.stateStore.Store.StartTransaction(ctx, "populating inputs"); err != nil {
		return nil, err
	}
	defer func() {
		if err := r.stateStore.Store.CommitTransaction(ctx); err != nil {
			log.Error("failed to commit transaction", "error", err)
		}
	}()

	nr, err := r.UnfilteredNextRun(ctx)
	if err != nil {
		return nil, err
	}

	log.Info("Filtering pending runs", "runs", nr)

	out := make(map[refs.Ref]*models.Run)

	for rr, run := range nr {
		missing, err := r.PopulateInputs(ctx, rr, run.Functions[len(run.Functions)-1])
		if err != nil {
			log.Error("failed to populate inputs", "function", rr.String(), "error", err)
			return nil, err
		}
		if len(missing) == 0 {
			out[rr] = run
		} else {
			log.Info("function missing inputs", "function", rr.String(), "missing", missing)
		}
	}

	log.Info("Filtered runs", "count", len(out))

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
		runCtx  map[string]context.Context = make(map[string]context.Context)
		runSpan map[string]trace.Span      = make(map[string]trace.Span)
	)

	var (
		nr  map[refs.Ref]*models.Run
		err error
	)
	for nr, err = r.FilteredNextRun(ctx); len(nr) > 0; nr, err = r.FilteredNextRun(ctx) {
		if err != nil {
			return fmt.Errorf("failed to get next functions: %w", err)
		}

		for runRef, run := range nr {
			taskName := path.Join(string(runRef.SubPathType), strings.Split(runRef.SubPath, "/")[0])

			if _, ok := runSpan[taskName]; !ok {
				runCtx[taskName], runSpan[taskName] = tracer.Start(ctx, fmt.Sprintf("RUN %s", taskName))
			}

			var result sdk.Result
			result, err = r.Run(runCtx[taskName], func(log sdk.Log) {
				logger(runRef, log)
			}, runRef, run)
			if err != nil {
				log.Error("failed to execute run", "run", runRef.String(), "error", err)
				return fmt.Errorf("failed to execute run %s: %w", runRef.String(), err)
			}

			if result.Err != nil {
				logger(runRef, sdk.Log{
					Timestamp: time.Now(),
					Message:   result.Err.Error(),
				})
			}

			log.Info("finished executing run", "run", runRef.String())

			// Check if the run or phase is now complete
			runStatus, err := r.stateStore.GetRunStatus(ctx, runRef)
			if err != nil {
				return fmt.Errorf("failed to get run status: %w", err)
			}
			if runStatus != models.StatusRunning && runStatus != models.StatusPending {
				runSpan[taskName].End()
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

		// Create an incremented run ref for the retry
		runRef := ReduceToRunRef(jobRef)
		runRefStr := runRef.String()
		lastSlashIndex := strings.LastIndex(runRefStr, "/")
		if lastSlashIndex == -1 {
			return fmt.Errorf("invalid run ref format: %s", runRefStr)
		}
		runRefPrefix := runRefStr[:lastSlashIndex+1] // includes the trailing slash

		incrementedRunPath, err := refstore.IncrementPath(ctx, r.stateStore.Store, runRefPrefix)
		if err != nil {
			return fmt.Errorf("failed to increment run ref: %w", err)
		}

		incrementedRunRef, err := refs.Parse(incrementedRunPath)
		if err != nil {
			return fmt.Errorf("failed to parse incremented run ref: %w", err)
		}

		// Update the parent run status to failed_retried as well
		originalRunRef := ReduceToRunRef(jobRef)
		if err := saveStatus(ctx, r.stateStore.Store, originalRunRef, models.StatusFailedRetried); err != nil {
			return fmt.Errorf("failed to update original run status: %w", err)
		}

		fn := job.Functions[0]

		// Reset the function state for retry - create a clean copy
		retryFn := models.Function{
			Fn:           fn.Fn,
			Dependencies: fn.Dependencies,
			Inputs:       fn.Inputs,
		}

		// Initialize the state for the new run using the standalone function
		if err := InitializeRun(ctx, r.stateStore.Store, incrementedRunRef, &retryFn); err != nil {
			return fmt.Errorf("failed to initialize retry run: %w", err)
		}

		// The retry function is ready to execute since inputs were already populated
		// in the previous failed run. It will be picked up by the normal execution flow.
		log.Info("retry run created and ready for execution",
			"ref", incrementedRunRef.String(),
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
	// Create all our functions up front
	functions, err := r.stateStore.SDKPackageToFunctions(ctx, r.pkg)
	if err != nil {
		return fmt.Errorf("failed to create runs: %w", err)
	}

	for runRef, fn := range functions {
		// Only replicate the first run
		// This will capture new environments
		if path.Base(runRef.String()) != "1" {
			continue
		}

		log.Info("Creating run", "ref", runRef.String())

		err = r.stateStore.InitializeFunction(ctx, models.Run{
			Type:    models.RunTypeUp,
			Release: r.stateStore.ReleaseRef,
		}, runRef, fn)
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

	// Add inputs to function if this is the first one
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

func (r *ReleaseTracker) updateIntent(ctx context.Context, taskRef refs.Ref, run *models.Run) error {
	// We only want intents for deployments
	if taskRef.SubPathType != refs.SubPathTypeDeploy {
		return nil
	}
	// Only update the first time we run a deployment
	if len(run.Functions) != 1 {
		return nil
	}

	fn := run.Functions[0]

	intentRef := taskRef.MakeIntent().SetVersion("")
	intentRef = intentRef.SetSubPath(path.Dir(intentRef.SubPath))
	intent := models.Intent{
		Type:    run.Type,
		Release: r.stateStore.ReleaseRef,
		Inputs:  fn.Inputs,
	}

	if err := r.intent.Set(ctx, intentRef.String(), intent); err != nil {
		return fmt.Errorf("failed to set intent state: %w", err)
	}

	return nil
}

func (r *ReleaseTracker) Run(
	ctx context.Context,
	logger sdk.Logger,
	runRef refs.Ref,
	run *models.Run,
) (sdk.Result, error) {
	log.Info("Run", "run", runRef.String())
	action := fmt.Sprintf("RUN %s", runRef.String())
	ctx, span := tracer.Start(ctx, action)
	defer span.End()

	taskName := strings.Split(runRef.SubPath, "/")[0]

	span.SetAttributes(
		attribute.String(AttributeCICDPipelineTaskName, taskName),
		attribute.String(AttributeCICDPipelineTaskRunType, "run"),
	)

	log.Info("executing run", "run", runRef.String())

	if err := r.stateStore.Store.StartTransaction(ctx, "execution started\n\n"+runRef.String()); err != nil {
		return sdk.Result{}, fmt.Errorf("failed to start transaction: %w", err)
	}

	// Set status of work
	if err := saveStatus(ctx, r.stateStore.Store, runRef, models.StatusRunning); err != nil {
		return sdk.Result{}, fmt.Errorf("failed to save status: %w", err)
	}

	// Set intent if appropriate
	if err := r.updateIntent(ctx, runRef, run); err != nil {
		return sdk.Result{}, fmt.Errorf("failed to update intent state: %w", err)
	}
	if err := r.stateStore.Store.CommitTransaction(ctx); err != nil {
		log.Error("failed to commit transaction", "error", err)
	}

	if err := r.stateStore.Store.StartTransaction(ctx, "execution finished\n\n"+runRef.String()); err != nil {
		return sdk.Result{}, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err := r.stateStore.Store.CommitTransaction(ctx); err != nil {
			log.Error("failed to commit transaction", "error", err)
		}
	}()

	// Start from the final function in the run
	fn := run.Functions[len(run.Functions)-1]

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
			return sdk.Result{}, fmt.Errorf("failed to run function %s: %w", fn.Fn, err)
		}

		// If we completed but no result was provided, assume success.
		// This handles when someone forgot an empty done().
		if result.Err == nil && result.Done == nil && result.Next == nil {
			result.Done = &sdk.Done{}
		}

		// Record the result of this function to the state store
		if err := r.saveRunState(ctx, runRef, run, result, logs); err != nil {
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
			run.Outputs = result.Done.Outputs
			return result, nil
		}

		if result.Next != nil {
			nextFunction := &models.Function{
				Fn:     result.Next.Fn,
				Inputs: result.Next.Inputs,
			}
			if err := validateFunction(nextFunction); err != nil {
				return sdk.Result{}, fmt.Errorf("failed to validate function: %w", err)
			}

			nextFunction.Inputs, err = PopulateInputs(ctx, r.stateStore.Store, nextFunction.Inputs)
			if err != nil {
				return sdk.Result{}, fmt.Errorf("failed to populate inputs for %s: %w", nextFunction.Fn.Name, err)
			}
			run.Functions = append(run.Functions, nextFunction)

			missing := GetMissing(nextFunction.Inputs)
			if len(missing) > 0 {
				log.Info("Next function was missing inputs", "missing", missing)
				// Update state to ensure we capture the next function
				if err := r.saveRunState(ctx, runRef, run, result, logs); err != nil {
					return result, fmt.Errorf("failed to save run state: %w", err)
				}
				return result, nil
			}

			fn = nextFunction
		}
	}
	return sdk.Result{}, errors.New("next or done was not called")
}

func ResultToStatus(result sdk.Result) models.Status {
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

func (r *ReleaseTracker) updateLogs(ctx context.Context, runRef refs.Ref, logs []sdk.Log) error {
	logRef := runRef.JoinSubPath("logs")

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

func (r *ReleaseTracker) saveRunState(ctx context.Context, runRef refs.Ref, run *models.Run, result sdk.Result, logs []sdk.Log) error {
	// Append logs
	if err := r.updateLogs(ctx, runRef, logs); err != nil {
		return err
	}

	status := ResultToStatus(result)
	log.Info("Setting status", "ref", runRef.String(), "status", status)
	if err := saveStatus(ctx, r.stateStore.Store, runRef, status); err != nil {
		return fmt.Errorf("failed to save run status: %w", err)
	}

	if result.Done != nil {
		run.Outputs = result.Done.Outputs
	}

	if err := r.stateStore.Store.Set(ctx, runRef.String(), run); err != nil {
		return fmt.Errorf("failed to save run detail: %w", err)
	}

	// If the run completed successfully, record it as the most recent run ref
	if status != models.StatusComplete {
		return nil
	}

	if err := r.stateStore.AddTags(ctx, result.Done.Tags); err != nil {
		return fmt.Errorf("failed to add tags: %w", err)
	}

	log.Info("Setting run", "ref", runRef.String())
	if err := r.stateStore.Store.Set(ctx, runRef.String(), run); err != nil {
		return fmt.Errorf("failed to save run detail: %w", err)
	}
	taskRef, err := refs.Reduce(runRef.String(), GlobTask)
	if err != nil {
		return fmt.Errorf("failed to reduce run ref: %w", err)
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
		return fmt.Errorf("failed to save task: %w", err)
	}

	taskRefParsed, err := refs.Parse(taskRef)
	if err != nil {
		return fmt.Errorf("failed to parse task ref: %w", err)
	}
	latestReleaseTaskRef := taskRefParsed.MakeRelease().SetVersion("")
	if run.Type == models.RunTypeDown {
		if err := r.stateStore.Store.Unlink(ctx, latestReleaseTaskRef.String()); err != nil {
			return fmt.Errorf("failed to unlink task: %w", err)
		}
	} else {
		if err := r.stateStore.Store.Link(ctx, latestReleaseTaskRef.String(), taskRef); err != nil {
			return fmt.Errorf("failed to link task: %w", err)
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
