package work

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/gittools"
	"github.com/ocuroot/ocuroot/client"
	"github.com/ocuroot/ocuroot/client/local"
	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/client/tui/tuiwork"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"
	"go.starlark.net/starlark"

	librelease "github.com/ocuroot/ocuroot/lib/release"
)

func (w *InRepoWorker) InitTrackerFromStateRepo(ctx context.Context, ref refs.Ref, wd, storeRootPath string) error {
	fs, err := refstore.NewFSRefStore(storeRootPath, stateTags)
	if err != nil {
		return fmt.Errorf("failed to create fs ref store: %w", err)
	}

	readOnlyStore := refstore.NewReadOnlyStore(fs)
	repoRefs, err := readOnlyStore.Match(ctx, "**/-/repo.ocu.star/@")
	if err != nil {
		return fmt.Errorf("failed to match repo refs: %w", err)
	}
	if len(repoRefs) == 0 {
		return fmt.Errorf("no repos registered in store")
	}

	w.Tracker.State = readOnlyStore
	w.RepoName = "" // Should always be empty to force clones

	var errorsByRepo = make(map[string]error)
	for _, repoRef := range repoRefs {
		resolvedRepoRef, err := readOnlyStore.ResolveLink(ctx, repoRef)
		if err != nil {
			errorsByRepo[repoRef] = err
			continue
		}
		repoRefParsed, err := refs.Parse(resolvedRepoRef)
		if err != nil {
			errorsByRepo[repoRef] = err
			continue
		}
		globals, be, err := w.RepoConfigFromState(ctx, repoRefParsed.Repo)
		if err != nil {
			return fmt.Errorf("failed to get repo config: %w", err)
		}

		// Load globals from repo into settings
		w.Settings, err = LoadSettings(be, globals, os.Environ())
		if err != nil {
			return fmt.Errorf("failed to load settings: %w", err)
		}

		if ref.IsRelative() && !ref.IsEmpty() && !ref.Global {
			return fmt.Errorf("relative refs not supported for state repo (%v)", ref)
		}

		storeConfig := &sdk.Store{
			State:  *w.Settings.State,
			Intent: w.Settings.Intent,
		}

		state, intent, err := release.NewRefStore(
			storeConfig,
			ref.Repo,
			"",
		)
		if err != nil {
			return fmt.Errorf("failed to create ref store: %w", err)
		}

		w.Tracker = release.TrackerConfig{
			Commit:      "",
			RepoPath:    "",
			Ref:         ref,
			State:       state,
			Intent:      intent,
			StoreConfig: storeConfig,
		}

		// Load the most recent push index
		// For intent repos this will just contain the commit
		if w.RepoInfo.Type == client.RepoTypeIntent {
			err = w.Tracker.State.Get(
				ctx,
				intentCommitRecordRef,
				&w.Index,
			)
			if err != nil && !errors.Is(err, refstore.ErrRefNotFound) {
				return fmt.Errorf("failed to get push index: %w", err)
			}
		}

		return nil
	}

	return fmt.Errorf("failed to init tracker from state repo\n%v", errorsByRepo)
}

func (w *InRepoWorker) InitTrackerFromSourceRepo(ctx context.Context, ref refs.Ref, wd, repoRootPath string, saveConfig bool) error {
	re := tuiwork.GetRepoEvent(repoRootPath, ref, w.Tui, tuiwork.WorkStatusRunning)
	w.Tui.UpdateTask(re)
	tLog := tuiwork.TuiLoggerForRepo(w.Tui, repoRootPath, ref)

	// Create a backend that is just enough for loading repo config
	backend, be := local.BackendForRepo()
	globals, data, err := sdk.LoadRepo(
		ctx,
		sdk.NewFSResolver(os.DirFS(repoRootPath)),
		"repo.ocu.star",
		backend,
		func(thread *starlark.Thread, msg string) {
			log.Info(msg)
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
	)
	if err != nil {
		re := tuiwork.GetRepoEvent(repoRootPath, ref, w.Tui, tuiwork.WorkStatusFailed)
		w.Tui.UpdateTask(re)
		return fmt.Errorf("failed to load repo: %w", err)
	}
	// Load globals from repo into settings
	w.Settings, err = LoadSettings(be, globals, os.Environ())
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	w.RepoName = w.Settings.RepoAlias
	if w.RepoName == "" {
		repoURL, err := client.GetRepoURL(repoRootPath)
		if err != nil {
			return err
		}
		w.RepoName = repoURL
	}

	if ref.IsRelative() {
		wdRel, err := filepath.Rel(repoRootPath, wd)
		if err != nil {
			return err
		}

		baseRef := refs.Ref{
			Repo:     w.RepoName,
			Filename: wdRel,
		}
		ref, err = ref.RelativeTo(baseRef)
		if err != nil {
			return err
		}
	}

	// Complete the repo load now we have a full ref
	re = tuiwork.GetRepoEvent(repoRootPath, ref, w.Tui, tuiwork.WorkStatusDone)
	w.Tui.UpdateTask(re)

	if w.Settings.State == nil {
		return fmt.Errorf("state not specified in repo")
	}

	storeConfig := &sdk.Store{
		State:  *w.Settings.State,
		Intent: w.Settings.Intent,
	}

	state, intent, err := release.NewRefStore(
		storeConfig,
		ref.Repo,
		repoRootPath,
	)
	if err != nil {
		return fmt.Errorf("failed to create ref store: %w", err)
	}

	tc := release.TrackerConfig{
		Commit:      w.RepoInfo.Commit,
		RepoPath:    repoRootPath,
		Ref:         ref,
		State:       state,
		Intent:      intent,
		StoreConfig: storeConfig,
	}
	w.Tracker = tc

	if saveConfig && tc.Ref.Repo != "" {
		err = saveRepoConfig(ctx, tc, repoRootPath, w.RepoName, tc.Commit, data)
		if err != nil {
			log.Info("Failed to save repo config", "repo", tc.Ref.Repo, "repoRootPath", repoRootPath, "repoName", w.RepoName, "commit", tc.Commit, "err", err)
			return fmt.Errorf("saving repo config: %w", err)
		}
	}

	// Load the most recent push index
	err = w.Tracker.State.Get(
		ctx,
		fmt.Sprintf(
			"%v/-/repo.ocu.star/@/push/index",
			w.RepoName,
		),
		&w.Index,
	)
	if err != nil && !errors.Is(err, refstore.ErrRefNotFound) {
		return fmt.Errorf("failed to get push index: %w", err)
	}

	return nil
}

func (w *InRepoWorker) TrackerForNewRelease(ctx context.Context) (*librelease.ReleaseTracker, []sdk.Environment, error) {
	var err error

	tc := w.Tracker

	if tc.Ref.HasRelease() {
		return nil, nil, fmt.Errorf("release should not be specified")
	}

	tc.Ref, err = release.NextReleaseID(ctx, tc.State, tc.Ref)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get next release ID: %w", err)
	}

	backend, outputs := release.NewBackend(tc)

	configEvent := tuiwork.GetConfigEvent(tc.Ref, w.Tui, tuiwork.WorkStatusRunning, nil)
	w.Tui.UpdateTask(configEvent)
	tLog := tuiwork.TuiLoggerForConfig(w.Tui, tc.Ref, nil)

	config, err := local.ExecutePackageWithLogging(ctx, tc.RepoPath, tc.Ref, backend, func(thread *starlark.Thread, msg string) {
		tLog(sdk.Log{
			Timestamp: time.Now(),
			Message:   msg,
			Attributes: map[string]string{
				"thread":   thread.Name,
				"filename": thread.CallFrame(1).Pos.Filename(),
				"line":     fmt.Sprintf("%d", thread.CallFrame(1).Pos.Line),
				"col":      fmt.Sprintf("%d", thread.CallFrame(1).Pos.Col),
			},
		})
	})
	if err != nil {
		configEvent = tuiwork.GetConfigEvent(tc.Ref, w.Tui, tuiwork.WorkStatusFailed, nil)
		w.Tui.UpdateTask(configEvent)
		return nil, nil, fmt.Errorf("failed to load config for %v: %w", tc.Ref.String(), err)
	}

	configEvent = tuiwork.GetConfigEvent(tc.Ref, w.Tui, tuiwork.WorkStatusDone, config)
	w.Tui.UpdateTask(configEvent)

	if len(outputs.Environments) > 0 && config.Package != nil {
		return nil, nil, fmt.Errorf("environments and packages should not be declared in the same file")
	}

	if config.Package == nil {
		if len(outputs.Environments) > 0 {
			return nil, outputs.Environments, nil
		}
		return nil, nil, nil
	}

	tracker, err := librelease.NewReleaseTracker(ctx, config, config.Package, tc.Ref, tc.Intent, tc.State)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create release tracker: %w", err)
	}

	err = tracker.InitRelease(ctx, tc.Commit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to init release: %w", err)
	}

	return tracker, nil, nil
}

func (w *InRepoWorker) TrackerForExistingRelease(ctx context.Context) (*librelease.ReleaseTracker, error) {
	tc := w.Tracker

	backend, _ := release.NewBackend(tc)

	configEvent := tuiwork.GetConfigEvent(tc.Ref, w.Tui, tuiwork.WorkStatusRunning, nil)
	w.Tui.UpdateTask(configEvent)
	tLog := tuiwork.TuiLoggerForConfig(w.Tui, tc.Ref, nil)

	config, err := local.ExecutePackageWithLogging(ctx, tc.RepoPath, tc.Ref, backend, func(thread *starlark.Thread, msg string) {
		tLog(sdk.Log{
			Timestamp: time.Now(),
			Message:   msg,
			Attributes: map[string]string{
				"thread":   thread.Name,
				"filename": thread.CallFrame(1).Pos.Filename(),
				"line":     fmt.Sprintf("%d", thread.CallFrame(1).Pos.Line),
				"col":      fmt.Sprintf("%d", thread.CallFrame(1).Pos.Col),
			},
		})
	})
	if err != nil {
		configEvent := tuiwork.GetConfigEvent(tc.Ref, w.Tui, tuiwork.WorkStatusFailed, nil)
		w.Tui.UpdateTask(configEvent)
		return nil, fmt.Errorf("failed to load config %w", err)
	}

	configEvent = tuiwork.GetConfigEvent(tc.Ref, w.Tui, tuiwork.WorkStatusDone, config)
	w.Tui.UpdateTask(configEvent)

	if !tc.Ref.HasRelease() {
		return nil, fmt.Errorf("no release was specified")
	}

	if config.Package == nil {
		return nil, fmt.Errorf("package not found")
	}

	tracker, err := librelease.NewReleaseTracker(ctx, config, config.Package, tc.Ref, tc.Intent, tc.State)
	if err != nil {
		return nil, fmt.Errorf("failed to create release tracker: %w", err)
	}

	releaseSummary, err := tracker.GetReleaseInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get release info: %w", err)
	}

	if tc.Commit != releaseSummary.Commit {
		return nil, fmt.Errorf("release commit does not match current commit:\nTo check out the expected commit, run:\n\tgit checkout %v", releaseSummary.Commit)
	}

	return tracker, nil
}

func saveRepoConfig(ctx context.Context, tc release.TrackerConfig, repoPath, repoName, commit string, data []byte) (err error) {
	// Write the repo file to the state stores for later use
	repoRef, err := refs.Parse(fmt.Sprintf("%s/-/repo.ocu.star/@%s", repoName, commit))
	if err != nil {
		return fmt.Errorf("failed to parse repo ref: %w", err)
	}
	r, err := gittools.Open(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repo: %w", err)
	}

	remotes, err := r.Remotes()
	if err != nil {
		return fmt.Errorf("failed to get remotes: %w", err)
	}

	repoConfig := models.RepoConfig{
		Remotes: remotes,
		Source:  data,
	}

	err = tc.State.StartTransaction(ctx, "Save repo config")
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	err = tc.Intent.StartTransaction(ctx, "Save repo config")
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		// TODO: We need a way to revert a transaction
		if err == nil {
			stateErr := tc.State.CommitTransaction(ctx)
			intentErr := tc.Intent.CommitTransaction(ctx)
			if stateErr != nil && intentErr != nil {
				err = fmt.Errorf("failed to commit transaction: %w, %w", stateErr, intentErr)
			} else if stateErr != nil {
				err = fmt.Errorf("failed to commit transaction: %w", stateErr)
			} else if intentErr != nil {
				err = fmt.Errorf("failed to commit transaction: %w", intentErr)
			}
		}
	}()

	if err := tc.State.Get(ctx, repoRef.String(), &models.RepoConfig{}); err == refstore.ErrRefNotFound {
		if err = tc.State.Set(ctx, repoRef.String(), repoConfig); err != nil {
			return fmt.Errorf("saving state repo config: %w", err)
		}
		if err = tc.State.Link(ctx, repoRef.SetRelease("").String(), repoRef.String()); err != nil {
			return fmt.Errorf("linking current state repo state: %w", err)
		}
	}

	if err := tc.Intent.Get(ctx, repoRef.String(), &models.RepoConfig{}); err == refstore.ErrRefNotFound {
		if err := tc.Intent.Set(ctx, repoRef.String(), repoConfig); err != nil {
			return fmt.Errorf("saving intent repo config: %w", err)
		}
		if err := tc.Intent.Link(ctx, repoRef.SetRelease("").String(), repoRef.String()); err != nil {
			return fmt.Errorf("linking current intent repo state: %w", err)
		}
	}

	return nil
}
