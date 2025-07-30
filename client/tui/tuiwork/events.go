package tuiwork

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/tree"
	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/tui"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"

	librelease "github.com/ocuroot/ocuroot/lib/release"
)

func initEvent(ref refs.Ref, t tui.Tui) *TaskEvent {
	wr, err := librelease.WorkRefFromChainRef(ref)
	if err != nil {
		log.Error("failed to get work ref", "error", err)
		return nil
	}
	chainRef := librelease.ChainRefFromFunctionRef(ref)

	var out *TaskEvent = &TaskEvent{}

	tr, found := t.GetTaskByID(wr.String())
	if found {
		out.Old, _ = tr.(*Task)
	}

	if out.Old != nil {
		newTask := *out.Old
		out.New = &newTask
	}
	if out.New == nil {
		name := strings.Split(chainRef.SubPath, "/")[0]
		if chainRef.SubPathType == refs.SubPathTypeDeploy {
			name = fmt.Sprintf("deploy to %s", name)
		}

		out.New = &Task{
			TaskID: wr.String(),
			Name:   name,
			Status: WorkStatusPending,
		}
	}

	return out
}

func updateStatus(ctx context.Context, store refstore.Store, ref refs.Ref, ev *TaskEvent) {
	chainRef := librelease.ChainRefFromFunctionRef(ref)
	chainStatus, err := librelease.GetFunctionChainStatusFromFunctions(ctx, store, chainRef)
	if err != nil {
		log.Error("failed to get function chain status", "chainRef", ref.String(), "error", err)
		return
	}

	var status WorkStatus
	switch chainStatus {
	case models.SummarizedStatusPending:
		status = WorkStatusPending
	case models.SummarizedStatusRunning:
		status = WorkStatusRunning
	case models.SummarizedStatusComplete:
		status = WorkStatusDone
	case models.SummarizedStatusFailed:
		status = WorkStatusFailed
	default:
		status = WorkStatusDone
	}

	ev.New.Status = status
}

func TuiLogger(tuiWork tui.Tui) func(fnRef refs.Ref, msg sdk.Log) {
	return func(fnRef refs.Ref, msg sdk.Log) {
		wr, err := librelease.WorkRefFromChainRef(fnRef)
		if err != nil {
			log.Error("failed to get work ref", "error", err)
			return
		}
		log.Info("function log", "ref", wr.String(), "msg", msg)

		out := initEvent(fnRef, tuiWork)
		out.New.Logs = append(out.New.Logs, msg.Message)

		tuiWork.UpdateTask(out)
	}
}

func WatchForChainUpdates(store refstore.Store, tuiWork tui.Tui) refstore.Store {
	updater := tuiStateChange(store, tuiWork)

	store, err := refstore.ListenToStateChanges(
		func(ctx context.Context, ref string) {
			r, err := refs.Parse(ref)
			if err != nil {
				log.Error("failed to parse ref", "error", err)
				return
			}

			// Ignore deleted refs
			// if err := store.Get(ctx, ref, nil); err == refstore.ErrRefNotFound {
			// 	return
			// }

			updater(r)
		},
		store,
		"**/{call,deploy}/*/*/status/*",
	)
	if err != nil {
		log.Error("failed to listen to state changes", "error", err)
		return store
	}

	return store
}

func tuiStateChange(store refstore.Store, tuiWork tui.Tui) func(ref refs.Ref) {
	return func(ref refs.Ref) {
		ctx := context.Background()
		chainRef := librelease.ChainRefFromFunctionRef(ref)
		wr, err := librelease.WorkRefFromChainRef(chainRef)
		if err != nil {
			log.Error("failed to get work ref", "ref", ref.String(), "error", err)
			return
		}

		out := initEvent(chainRef, tuiWork)
		updateStatus(ctx, store, chainRef, out)

		var message string
		if out.New.Status == WorkStatusDone || out.New.Status == WorkStatusPending {

			// Get chain outputs and render as a message
			var chainWork models.Work
			if err := store.Get(ctx, chainRef.String(), &chainWork); err != nil {
				log.Error("failed to get chain work", "ref", chainRef.String(), "error", err)
				return
			}

			if out.New.Status == WorkStatusDone && len(chainWork.Outputs) > 0 {
				outputs := tree.Root("Outputs")
				for k, v := range chainWork.Outputs {
					outputs = outputs.Child(
						tree.Root(
							fmt.Sprintf("%s#output/%s", wr.String(), k),
						).Child(v),
					)
				}
				message += outputs.String()
			}

			if out.New.Status == WorkStatusPending {
				var fn librelease.FunctionState
				if err := store.Get(ctx, chainWork.Entrypoint.String(), &fn); err != nil {
					log.Error("failed to get function summary", "chainRef", chainRef.String(), "entrypoint", chainWork.Entrypoint.String(), "error", err)
					return
				}

				hasPending := false
				pendingInputs := tree.Root("Pending Inputs")
				for _, v := range fn.Current.Inputs {
					retrieved, err := librelease.RetrieveInput(ctx, store, v)
					if err != nil {
						log.Error("failed to retrieve input", "ref", v.Ref.String(), "error", err)
						return
					}

					if retrieved.Default == nil && retrieved.Value == nil {
						hasPending = true
						pendingInputs = pendingInputs.Child(v.Ref)
					}
				}
				if hasPending {
					message += pendingInputs.String()
				}
			}
			out.New.Message = message
		}

		if out.New.Status == WorkStatusRunning {
			if out.Old == nil || out.Old.Status != WorkStatusRunning {
				out.New.StartTime = time.Now()
			}
		}
		if out.New.Status == WorkStatusDone || out.New.Status == WorkStatusFailed {
			if out.Old == nil || (out.Old.Status != WorkStatusDone && out.Old.Status != WorkStatusFailed) {
				out.New.EndTime = time.Now()
			}
		}

		tuiWork.UpdateTask(out)
	}
}
