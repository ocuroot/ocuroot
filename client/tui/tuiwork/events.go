package tuiwork

import (
	"context"
	"fmt"
	"path"
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
	ctx := context.TODO()
	var err error

	// tr, err := librelease.ReduceToTaskRef(ref)
	// if err != nil {
	// 	log.Error("failed to get task ref", "error", err)
	// 	return nil
	// }
	runRef := librelease.ReduceToRunRef(ref)

	var out *TaskEvent = &TaskEvent{}

	ttr, found := t.GetTaskByID(runRef.String())
	if found {
		out.Old, _ = ttr.(*Task)
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
		name := strings.Split(runRef.SubPath, "/")[0]
		if runRef.SubPathType == refs.SubPathTypeDeploy {
			var run models.Run
			if store != nil {
				err = store.Get(ctx, runRef.String(), &run)
				if err != nil {
					log.Error("failed to get run", "error", err)
				} else {
					if run.Type == models.RunTypeDown {
						name = fmt.Sprintf("remove from %s", name)
					} else {
						name = fmt.Sprintf("deploy to %s", name)
					}
				}
			} else {
				log.Error("failed to get run", "error", "no store")
			}
		}
		name += fmt.Sprintf(" [%s]", path.Base(runRef.SubPath))

		out.New = &Task{
			RunRef:       runRef,
			Name:         name,
			Status:       WorkStatusPending,
			CreationTime: time.Now(),

			Store:  store,
			JobRef: runRef,
		}
	}

	return out
}

func updateStatus(ctx context.Context, store refstore.Store, ref refs.Ref, ev *TaskEvent) {
	runRef := librelease.ReduceToRunRef(ref)
	runStatus, err := librelease.GetRunStatus(ctx, store, runRef)
	if err != nil {
		log.Error("failed to get work status", "runRef", ref.String(), "error", err)
		return
	}

	var status WorkStatus
	switch runStatus {
	case models.StatusPending:
		status = WorkStatusPending
	case models.StatusRunning:
		status = WorkStatusRunning
	case models.StatusComplete:
		status = WorkStatusDone
	case models.StatusFailed:
		status = WorkStatusFailed
	default:
		status = WorkStatusDone
	}

	ev.New.Status = status
}

func TuiLogger(tuiWork tui.Tui) func(fnRef refs.Ref, msg sdk.Log) {
	return func(fnRef refs.Ref, msg sdk.Log) {
		wr, err := librelease.ReduceToTaskRef(fnRef)
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

func WatchForJobUpdates(ctx context.Context, store refstore.Store, tuiWork tui.Tui) refstore.Store {
	updater := tuiStateChange(ctx, store, tuiWork)

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
		"**/{task,deploy}/*/*/status/*",
	)
	if err != nil {
		log.Error("failed to listen to state changes", "error", err)
		return store
	}

	return store
}

func tuiStateChange(ctx context.Context, store refstore.Store, tuiWork tui.Tui) func(ref refs.Ref) {
	return func(ref refs.Ref) {
		runRef := librelease.ReduceToRunRef(ref)

		out := initEvent(runRef, tuiWork, store)
		updateStatus(ctx, store, runRef, out)

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
