package work

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/tui/tuiwork"
	"github.com/ocuroot/ocuroot/refs"
)

func (w *Worker) PushWork(ctx context.Context) ([]Work, error) {
	files, err := w.RepoInfo.GetReleaseConfigFiles()
	if err != nil {
		return nil, err
	}

	var out []Work
	for _, file := range files {
		fileRef, err := refs.Parse(fmt.Sprintf("./-/%v", file))
		if err != nil {
			return nil, err
		}

		fileRef, err = fileRef.RelativeTo(w.Tracker.Ref)
		if err != nil {
			return nil, err
		}

		out = append(out, Work{
			Commit:   w.RepoInfo.Commit,
			WorkType: WorkTypeRelease,
			Ref:      fileRef,
		})
	}

	return out, nil
}

func (w *Worker) Push(ctx context.Context) error {
	if !w.RepoInfo.IsSource {
		return fmt.Errorf("state repos currently not supported")
	}

	pushWork, err := w.PushWork(ctx)
	if err != nil {
		return err
	}

	if len(pushWork) == 0 {
		log.Info("No push work")
		return nil
	}

	if err := w.ExecuteWorkInCleanWorktrees(ctx, pushWork); err != nil {
		return err
	}

	return w.Cascade(ctx)
}

func (w *Worker) startRelease(ctx context.Context, ref refs.Ref) error {
	w.Tracker.Ref = ref
	tracker, environments, err := w.TrackerForNewRelease(ctx)
	if err != nil {
		return err
	}

	if tracker == nil {
		for _, env := range environments {
			// Establishing intent for environment
			intentRef := "@/environment/" + string(env.Name)
			if err := w.Tracker.Intent.Set(ctx, intentRef, env); err != nil {
				return err
			}

			pr, err := refs.Parse(intentRef)
			if err != nil {
				return err
			}

			if err := w.ApplyIntent(ctx, pr); err != nil {
				return err
			}
		}

		if err := w.Cascade(ctx); err != nil {
			return err
		}
		return nil
	}

	err = tracker.RunToPause(
		ctx,
		tuiwork.TuiLogger(w.Tui),
	)
	if err != nil {
		return err
	}

	if err := w.Cascade(ctx); err != nil {
		return err
	}

	return nil
}
