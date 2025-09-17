package work

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/local"
	"github.com/ocuroot/ocuroot/client/tui/tuiwork"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"
	"go.starlark.net/starlark"
)

func (w *Worker) TriggerWork(ctx context.Context, todos []Work) error {
	type repoCommit struct {
		Repo   string
		Commit string
	}
	var repos = make(map[repoCommit]struct{})
	for _, todo := range todos {
		if todo.Commit == "" {
			continue
		}
		repos[repoCommit{
			Repo:   todo.Ref.Repo,
			Commit: todo.Commit,
		}] = struct{}{}
	}

	for repoCommit := range repos {
		if err := w.TriggerCommit(ctx, repoCommit.Repo, repoCommit.Commit); err != nil {
			log.Error("Failed to trigger run", "repo", repoCommit.Repo, "commit", repoCommit.Commit, "error", err)
		}
	}
	return nil
}

func (w *Worker) TriggerAll(ctx context.Context) error {
	log.Info("Triggering in intent mode")
	// Match repo config files to identify unique repos
	mr := "**/-/repo.ocu.star/@"
	repo, err := w.Tracker.State.Match(
		ctx,
		mr)
	if err != nil {
		return fmt.Errorf("failed to match refs: %w", err)
	}

	log.Info("Repo matches", "count", len(repo), "repo", repo)

	for _, ref := range repo {
		resolvedRepo, err := w.Tracker.State.ResolveLink(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to resolve repo ref (%v): %w", ref, err)
		}
		pr, err := refs.Parse(resolvedRepo)
		if err != nil {
			return fmt.Errorf("failed to parse resolved repo ref (%v): %w", resolvedRepo, err)
		}
		commit := pr.Release

		if err := w.TriggerCommit(ctx, pr.Repo, string(commit)); err != nil {
			fmt.Println("Failed to trigger run against " + ref + ": " + err.Error())
		}
	}

	return nil
}

func (w *Worker) TriggerCommit(ctx context.Context, repo, commit string) error {
	tuiEvent := tuiwork.GetTriggerEvent(repo, commit, w.Tui, tuiwork.TriggerStatusRunning, w.Tracker)
	w.Tui.UpdateTask(tuiEvent)

	tLog := tuiwork.TuiLoggerForTrigger(w.Tui, repo, commit, w.Tracker)

	configRef := repo + "/-/repo.ocu.star/@" + commit

	configWithCommit, err := w.Tracker.State.ResolveLink(ctx, configRef)
	if err != nil {
		tuiEvent := tuiwork.GetTriggerEvent(repo, commit, w.Tui, tuiwork.TriggerStatusFailed, w.Tracker)
		w.Tui.UpdateTask(tuiEvent)
		return fmt.Errorf("failed to resolve config ref (%v): %w", configRef, err)
	}

	log.Info("Triggering work for repo", "ref", configWithCommit)
	var repoConfig models.RepoConfig
	if err := w.Tracker.State.Get(ctx, configWithCommit, &repoConfig); err != nil {
		tuiEvent := tuiwork.GetTriggerEvent(repo, commit, w.Tui, tuiwork.TriggerStatusFailed, w.Tracker)
		w.Tui.UpdateTask(tuiEvent)
		return fmt.Errorf("failed to get repo config (%v): %w", configWithCommit, err)
	}

	backend, be := local.BackendForRepo()

	_, err = sdk.LoadRepoFromBytes(
		ctx,
		sdk.NewNullResolver(),
		"repo.ocu.star",
		repoConfig.Source,
		backend,
		func(thread *starlark.Thread, msg string) {},
	)
	if err != nil {
		tuiEvent := tuiwork.GetTriggerEvent(repo, commit, w.Tui, tuiwork.TriggerStatusFailed, w.Tracker)
		w.Tui.UpdateTask(tuiEvent)
		return fmt.Errorf("failed to load repo: %w", err)
	}

	if be.RepoTrigger != nil {
		log.Info("Executing repo trigger", "ref", configWithCommit)

		thread := &starlark.Thread{
			Name: "repo-trigger",
			Print: func(thread *starlark.Thread, msg string) {
				log.Info("Repo trigger", "msg", msg)
				cf := thread.CallFrame(1)
				tLog(sdk.Log{
					Timestamp: time.Now(),
					Message:   msg,
					Attributes: map[string]string{
						"thread":   thread.Name,
						"filename": cf.Pos.Filename(),
						"line":     fmt.Sprintf("%d", cf.Pos.Line),
						"col":      fmt.Sprintf("%d", cf.Pos.Col),
					},
				},
				)
			},
		}
		_, err = starlark.Call(thread, be.RepoTrigger, starlark.Tuple{starlark.String(commit)}, nil)
		if err != nil {
			tuiEvent := tuiwork.GetTriggerEvent(repo, commit, w.Tui, tuiwork.TriggerStatusFailed, w.Tracker)
			w.Tui.UpdateTask(tuiEvent)
			return fmt.Errorf("failed to call repo trigger: %w", err)
		}
	} else {
		tLog(sdk.Log{
			Timestamp: time.Now(),
			Message:   "No repo trigger found",
		})
		tuiEvent := tuiwork.GetTriggerEvent(repo, commit, w.Tui, tuiwork.TriggerStatusNoTrigger, w.Tracker)
		w.Tui.UpdateTask(tuiEvent)
		return nil
	}

	tuiEvent = tuiwork.GetTriggerEvent(repo, commit, w.Tui, tuiwork.TriggerStatusDone, w.Tracker)
	w.Tui.UpdateTask(tuiEvent)

	return nil
}
