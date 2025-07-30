package tuiwork

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/tui"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"

	librelease "github.com/ocuroot/ocuroot/lib/release"
)

func initEvent(ref refs.Ref, t tui.Tui, store refstore.Store) *TaskEvent {
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

		if store != nil {
			out.Old.Store = store
			out.New.Store = store
		}
	}
	if out.New == nil {
		name := strings.Split(chainRef.SubPath, "/")[0]
		if chainRef.SubPathType == refs.SubPathTypeDeploy {
			name = fmt.Sprintf("deploy to %s", name)
		}

		out.New = &Task{
			TaskID:       wr.String(),
			Name:         name,
			Status:       WorkStatusPending,
			CreationTime: time.Now(),

			Store:    store,
			ChainRef: chainRef,
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

		out := initEvent(fnRef, tuiWork, nil)
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

		out := initEvent(chainRef, tuiWork, store)
		updateStatus(ctx, store, chainRef, out)

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
