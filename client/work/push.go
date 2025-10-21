package work

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	libglob "github.com/gobwas/glob"
	"github.com/ocuroot/ocuroot/client"
	"github.com/ocuroot/ocuroot/client/tui/tuiwork"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/store/models"
)

const intentCommitRecordRef = "@/push/intent_commit"

func (w *InRepoWorker) PushWork(ctx context.Context) ([]Work, error) {
	if w.Index == nil {
		// Add all configs to the index
		w.Index = &models.PushIndex{
			Commit:         w.RepoInfo.Commit,
			PreviousCommit: "",
		}
	} else {
		log.Info("Using push index", "index", w.Index)
		if w.Index.Commit != w.RepoInfo.Commit {
			w.Index.PreviousCommit = w.Index.Commit
			w.Index.Commit = w.RepoInfo.Commit
		}
	}

	if w.RepoInfo.Type == client.RepoTypeState {
		return nil, fmt.Errorf("state repos currently not supported")
	}

	var out []Work
	var err error
	if w.RepoInfo.Type == client.RepoTypeIntent {
		out, err = w.pushWorkFromIntentRepo(ctx)
		if err != nil {
			return nil, err
		}
	}

	if w.RepoInfo.Type == client.RepoTypeSource {
		out, err = w.pushWorkFromSourceRepo(ctx)
		if err != nil {
			return nil, err
		}
	}

	out, err = w.filterReleaseWorkByFile(out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (w *InRepoWorker) filterReleaseWorkByFile(in []Work) ([]Work, error) {
	// Filter work by ignore patterns
	var ignoreGlobs []libglob.Glob
	for _, ignore := range w.Settings.ReleaseIgnore {
		g, err := libglob.Compile(ignore)
		if err != nil {
			return nil, fmt.Errorf("glob %v %q: %w", []rune(ignore), ignore, err)
		}
		ignoreGlobs = append(ignoreGlobs, g)
	}

	var filteredOut []Work
	for _, work := range in {
		var shouldIgnore bool
		for _, ignore := range ignoreGlobs {
			if ignore.Match(work.Ref.Filename) {
				shouldIgnore = true
				break
			}
		}
		if shouldIgnore {
			continue
		}
		filteredOut = append(filteredOut, work)
	}

	return filteredOut, nil
}

func (w *InRepoWorker) pushWorkFromSourceRepo(ctx context.Context) ([]Work, error) {
	var (
		files []string
		err   error
	)
	if w.Index.ReleaseConfigs == nil {
		w.Index.ReleaseConfigs = make(map[string]models.ReleaseConfig)
	}
	if w.Index.PreviousCommit == "" {
		log.Info("No previous commit, using all release config files")
		files, err = w.RepoInfo.GetReleaseConfigFiles()
		if err != nil {
			return nil, err
		}
	} else {
		changedFiles, err := w.getChangedFiles(ctx)
		if err != nil {
			return nil, err
		}

		log.Info("Changed files", "files", changedFiles)

		fileSet := make(map[string]struct{})

		changeIndex := make(map[string]struct{})
		for _, f := range changedFiles {
			changeIndex[f] = struct{}{}

			// Release any changed config files
			if strings.HasSuffix(f, ".ocu.star") {
				fileSet[f] = struct{}{}
			}
		}

		// TODO: This is a bit inefficient, and probably needs to
		// use a more trie-like structure
		for f, cfg := range w.Index.ReleaseConfigs {
			// Re-release if any of the watch files have changed
			for _, watchFile := range cfg.WatchFiles {
				for _, changedFile := range changedFiles {
					if strings.HasPrefix(changedFile, watchFile) {
						fileSet[f] = struct{}{}
						break
					}
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

func (w *InRepoWorker) pushWorkFromIntentRepo(ctx context.Context) ([]Work, error) {
	if w.Index.PreviousCommit == "" {
		log.Info("No previous commit, will analyze all state")
		work, err := w.Diff(ctx, IdentifyWorkRequest{})
		if err != nil {
			return nil, err
		}
		return work, nil
	}
	log.Info("Previous commit, will analyze changed files")
	changes, err := w.getChangedFiles(ctx)
	if err != nil {
		return nil, err
	}

	var out []Work
	for _, change := range changes {
		r, err := refs.Parse(change)
		if err != nil {
			return nil, err
		}
		workItem := Work{
			Commit: "",
			Ref:    r,
		}

		_, err = os.Stat(path.Join(w.RepoInfo.Root, change))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if os.IsNotExist(err) {
			workItem.WorkType = WorkTypeDelete
		} else {
			stateMatches, err := w.Tracker.State.Match(ctx, change)
			if err != nil {
				return nil, err
			}
			if len(stateMatches) == 0 {
				workItem.WorkType = WorkTypeCreate
			} else {
				stateRef, err := refs.Parse(stateMatches[0])
				if err != nil {
					return nil, err
				}
				changed, err := compareIntent(ctx, w.Tracker.State, w.Tracker.Intent, r, stateRef)
				if err != nil {
					return nil, err
				}
				if changed {
					workItem.WorkType = WorkTypeUpdate
				}
			}
		}

		out = append(out, workItem)
	}

	return out, nil
}

func (w *InRepoWorker) Push(ctx context.Context) error {
	if len(w.RepoInfo.UncommittedChanges) > 0 {
		return fmt.Errorf(
			"you have uncommitted changes, please commit or stash them before running push\n\nChanged files:\n%v",
			strings.Join(w.RepoInfo.UncommittedChanges, "\n"),
		)
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

	if w.Index != nil && w.RepoInfo.Type == client.RepoTypeSource {
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

	if w.Index != nil && w.RepoInfo.Type == client.RepoTypeIntent {
		err = w.Tracker.State.Set(ctx, intentCommitRecordRef, w.Index)
		if err != nil {
			return fmt.Errorf("failed to set push index: %w", err)
		}
	}

	return nil
}

func (w *InRepoWorker) getChangedFiles(ctx context.Context) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", w.Index.PreviousCommit, w.Index.Commit)
	cmd.Dir = w.RepoInfo.Root
	out, err := cmd.Output()
	if err != nil {
		log.Error("Failed to get changed files", "error", err, "command", cmd.String())
		return nil, err
	}

	changedFiles := strings.Split(string(out), "\n")

	if len(w.RepoInfo.UncommittedChanges) > 0 {
		changedFiles = append(changedFiles, w.RepoInfo.UncommittedChanges...)
	}

	// Deduplicate changedFiles
	seen := make(map[string]struct{}, len(changedFiles))
	for _, file := range changedFiles {
		seen[file] = struct{}{}
	}
	changedFiles = make([]string, 0, len(seen))
	for file := range seen {
		changedFiles = append(changedFiles, file)
	}
	sort.Strings(changedFiles)

	return changedFiles, nil
}

func (w *InRepoWorker) getWatchFiles(ctx context.Context, work Work) ([]string, error) {
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

func (w *InRepoWorker) startRelease(ctx context.Context, ref refs.Ref) error {
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
