package tuiwork

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/tui"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"

	librelease "github.com/ocuroot/ocuroot/lib/release"
)

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
