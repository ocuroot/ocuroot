package work

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/tui/tuiwork"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/store/models"
)

func (w *Worker) PushWork(ctx context.Context) ([]Work, error) {
	var (
		files []string
		err   error
	)
	if w.Index == nil {
		// Add all configs to the index
		w.Index = &models.PushIndex{
			Commit:         w.RepoInfo.Commit,
			PreviousCommit: "",
			ReleaseConfigs: make(map[string]models.ReleaseConfig),
		}
	} else {
		log.Info("Using push index", "index", w.Index)
		if w.Index.Commit != w.RepoInfo.Commit {
			w.Index.PreviousCommit = w.Index.Commit
			w.Index.Commit = w.RepoInfo.Commit
		}
	}

	if w.Index.PreviousCommit == "" {
		log.Info("No previous commit, using all release config files")
		files, err = w.RepoInfo.GetReleaseConfigFiles()
		if err != nil {
			return nil, err
		}
	} else {
		changedFiles, err := w.GetChangedFiles(ctx)
		if err != nil {
			return nil, err
		}

		log.Info("Changed files", "files", changedFiles)

		fileSet := make(map[string]struct{})

		changeIndex := make(map[string]struct{})
		for _, f := range changedFiles {
			changeIndex[f] = struct{}{}

			// Release any net-new config files
			if strings.HasSuffix(f, ".ocu.star") {
				fileSet[f] = struct{}{}
			}
		}

		for f, cfg := range w.Index.ReleaseConfigs {
			// Always release if there are no watch files
			if len(cfg.WatchFiles) == 0 {
				fileSet[f] = struct{}{}
				continue
			}

			// Always re-release if the release config file has changed
			if _, ok := changeIndex[f]; ok {
				fileSet[f] = struct{}{}
				continue
			}
			// Otherwise, re-release if any of the watch files have changed
			for _, watchFile := range cfg.WatchFiles {
				if _, ok := changeIndex[watchFile]; ok {
					fileSet[f] = struct{}{}
					break
				}
			}
		}

		files = make([]string, 0, len(fileSet))
		for f := range fileSet {
			files = append(files, f)
		}
	}
	sort.Strings(files)

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

func (w *Worker) GetChangedFiles(ctx context.Context) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", w.Index.PreviousCommit, w.Index.Commit)
	cmd.Dir = w.RepoInfo.Root
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	changedFiles := strings.Split(string(out), "\n")
	return changedFiles, nil
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

	err = w.Cascade(ctx)
	if err != nil {
		return err
	}

	if w.Index != nil {
		// Build up the index
		for _, work := range pushWork {
			watchFiles, err := w.getWatchFiles(ctx, work)
			if err != nil {
				return err
			}
			w.Index.ReleaseConfigs[work.Ref.String()] = models.ReleaseConfig{
				WatchFiles: watchFiles,
			}
		}

		err = w.Tracker.State.Set(ctx, fmt.Sprintf("%v/-/repo.ocu.star/@%v/push/index", w.RepoName, w.RepoInfo.Commit), w.Index)
		if err != nil {
			return fmt.Errorf("failed to set push index: %w", err)
		}

		err = w.Tracker.State.Link(ctx, fmt.Sprintf("%v/-/repo.ocu.star/@/push/index", w.RepoName), fmt.Sprintf("%v/-/repo.ocu.star/@%v/push/index", w.RepoName, w.RepoInfo.Commit))
		if err != nil {
			return fmt.Errorf("failed to link push index: %w", err)
		}
	}

	return nil
}

func (w *Worker) getWatchFiles(ctx context.Context, work Work) ([]string, error) {
	releaseMatch, err := w.Tracker.State.Match(ctx, fmt.Sprintf("%v/@*/commit/%v", work.Ref.String(), work.Commit))
	if err != nil {
		return nil, err
	}

	log.Info("Getting watch files", "releaseMatch", releaseMatch, "ref", work.Ref.String(), "commit", work.Commit)

	if len(releaseMatch) == 0 {
		return nil, nil
	}

	parsedRelease, err := refs.Parse(releaseMatch[0])
	if err != nil {
		return nil, err
	}

	allRuns, err := w.Tracker.State.Match(ctx, fmt.Sprintf("%v/{deploy,task}/*/*", parsedRelease.SetSubPath("").SetSubPathType("").String()))
	if err != nil {
		return nil, err
	}

	var watchFiles []string
	for _, run := range allRuns {
		var runData models.Run
		if err := w.Tracker.State.Get(ctx, run, &runData); err != nil {
			return nil, err
		}
		watchFiles = append(watchFiles, runData.WatchFiles...)
	}

	// Remove duplicates from the list of watch files
	seen := make(map[string]struct{})
	for _, file := range watchFiles {
		seen[file] = struct{}{}
	}
	watchFiles = make([]string, 0, len(seen))
	for file := range seen {
		watchFiles = append(watchFiles, file)
	}
	sort.Strings(watchFiles)

	return watchFiles, nil
}

func (w *Worker) startRelease(ctx context.Context, ref refs.Ref) error {
	w.Tracker.Ref = ref
	tracker, environments, err := w.TrackerForNewRelease(ctx)
	if err != nil {
		return err
	}

	// TODO: If this release has already been created,
	// continue or retry it

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
