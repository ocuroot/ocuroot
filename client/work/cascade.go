package work

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
)

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
