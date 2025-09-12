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

func initRunStateEvent(ref refs.Ref, t tui.Tui, store refstore.Store) *RunTaskEvent {
	ctx := context.TODO()
	var err error

	runRef := librelease.ReduceToRunRef(ref)

	var out *RunTaskEvent = &RunTaskEvent{}

	ttr, found := t.GetTaskByID(runRef.String())
	if found {
		out.Old, _ = ttr.(*RunTask)
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

		out.New = &RunTask{
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

func initCustomStateEvent(ref refs.Ref, t tui.Tui, store refstore.Store) *CustomStateTaskEvent {
	var out *CustomStateTaskEvent = &CustomStateTaskEvent{}

	ttr, found := t.GetTaskByID(ref.String())
	if found {
		out.Old, _ = ttr.(*CustomStateTask)
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
		name := ref.SubPath

		out.New = &CustomStateTask{
			Ref:   ref,
			Name:  name,
			Store: store,
		}
	}

	return out
}

func updateStatus(ctx context.Context, store refstore.Store, ref refs.Ref, ev *RunTaskEvent) {
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

		out := initRunStateEvent(fnRef, tuiWork, nil)
		out.New.Logs = append(out.New.Logs, msg.Message)

		tuiWork.UpdateTask(out)
	}
}

func WatchForStateUpdates(ctx context.Context, store refstore.Store, tuiWork tui.Tui) refstore.Store {
	runUpdater := tuiRunStatusChange(ctx, store, tuiWork)

	store, err := refstore.ListenToStateChanges(
		func(ctx context.Context, ref string) {
			r, err := refs.Parse(ref)
			if err != nil {
				log.Error("failed to parse ref", "error", err)
				return
			}

			runUpdater(r)
		},
		store,
		"**/{task,deploy}/*/*/status/*",
	)
	if err != nil {
		log.Error("failed to listen to state changes", "error", err)
		return store
	}

	customUpdater := tuiCustomStateChange(ctx, store, tuiWork)
	store, err = refstore.ListenToStateChanges(
		func(ctx context.Context, ref string) {
			r, err := refs.Parse(ref)
			if err != nil {
				log.Error("failed to parse ref", "error", err)
				return
			}

			customUpdater(r)
		},
		store,
		"**/@*/custom/*", "@/custom/*", "@/environment/*",
	)
	if err != nil {
		log.Error("failed to listen to state changes", "error", err)
		return store
	}

	return store
}

func tuiCustomStateChange(ctx context.Context, store refstore.Store, tuiWork tui.Tui) func(ref refs.Ref) {
	return func(ref refs.Ref) {
		runRef := librelease.ReduceToRunRef(ref)

		out := initCustomStateEvent(runRef, tuiWork, store)
		tuiWork.UpdateTask(out)
	}
}

func tuiRunStatusChange(ctx context.Context, store refstore.Store, tuiWork tui.Tui) func(ref refs.Ref) {
	return func(ref refs.Ref) {
		runRef := librelease.ReduceToRunRef(ref)

		out := initRunStateEvent(runRef, tuiWork, store)
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
