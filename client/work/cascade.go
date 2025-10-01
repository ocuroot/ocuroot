package work

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
)

func (w *Worker) Cascade(ctx context.Context) error {
	log.Info("Cascading work", "StateChanges", w.StateChanges, "IntentChanges", w.IntentChanges)
	for {
		followOn, err := w.IdentifyWork(ctx, IdentifyWorkRequest{
			IntentChanges: w.IntentChanges,
			StateChanges:  w.StateChanges,
		})
		if err != nil {
			return err
		}

		if len(followOn) == 0 {
			log.Info("No follow on work")
			break
		}

		log.Info("Executing follow on work", "count", len(followOn))
		if err := w.ExecuteWorkInCleanWorktrees(ctx, followOn); err != nil {
			return err
		}

	}

	return nil
}

func (w *Worker) RecordStateUpdates(ctx context.Context) error {

	stateListener, err := refstore.ListenToStateChanges(
		func(ctx context.Context, ref string) {
			r, err := refs.Parse(ref)
			if err != nil {
				log.Error("failed to parse ref", "error", err)
				return
			}
			w.StateChanges[r.String()] = struct{}{}
		},
		w.Tracker.State,
		"**",
	)
	w.Tracker.State = stateListener
	if err != nil {
		log.Error("failed to listen to state changes", "error", err)
		return err
	}

	intentListener, err := refstore.ListenToStateChanges(
		func(ctx context.Context, ref string) {
			r, err := refs.Parse(ref)
			if err != nil {
				log.Error("failed to parse ref", "error", err)
				return
			}
			w.IntentChanges[r.String()] = struct{}{}
		},
		w.Tracker.Intent,
		"**",
	)
	w.Tracker.Intent = intentListener
	if err != nil {
		log.Error("failed to listen to state changes", "error", err)
		return err
	}

	return nil
}
