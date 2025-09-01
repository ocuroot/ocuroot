package release

import (
	"context"
	"fmt"

	"github.com/ocuroot/ocuroot/client/local"
	"github.com/ocuroot/ocuroot/lib/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
)

type TrackerConfig struct {
	Commit      string
	RepoPath    string
	Ref         refs.Ref
	Store       refstore.Store
	StoreConfig *sdk.Store
}

func TrackerForNewRelease(ctx context.Context, tc TrackerConfig) (*release.ReleaseTracker, []sdk.Environment, error) {
	var err error

	if tc.Ref.ReleaseOrIntent.Type != refs.Unknown {
		return nil, nil, fmt.Errorf("release should not be specified")
	}

	tc.Ref, err = NextReleaseID(ctx, tc.Store, tc.Ref)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get next release ID: %w", err)
	}

	backend, outputs := NewBackend(tc)
	config, err := local.ExecutePackage(ctx, tc.RepoPath, tc.Ref, backend)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	if len(outputs.Environments) > 0 && config.Package != nil {
		return nil, nil, fmt.Errorf("environments and packages should not be declared in the same file")
	}

	if config.Package == nil {
		if len(outputs.Environments) > 0 {
			return nil, outputs.Environments, nil
		}
		return nil, nil, fmt.Errorf("package not found")
	}

	tracker, err := release.NewReleaseTracker(ctx, config, config.Package, tc.Ref, tc.Store)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create release tracker: %w", err)
	}

	err = tracker.InitRelease(ctx, tc.Commit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to init release: %w", err)
	}

	return tracker, nil, nil
}

func TrackerForExistingRelease(ctx context.Context, tc TrackerConfig) (*release.ReleaseTracker, error) {
	backend, _ := NewBackend(tc)
	config, err := local.ExecutePackage(ctx, tc.RepoPath, tc.Ref, backend)
	if err != nil {
		return nil, fmt.Errorf("failed to load config %w", err)
	}

	if tc.Ref.ReleaseOrIntent.Type != refs.Release {
		return nil, fmt.Errorf("no release was specified")
	}

	if config.Package == nil {
		return nil, fmt.Errorf("package not found")
	}

	tracker, err := release.NewReleaseTracker(ctx, config, config.Package, tc.Ref, tc.Store)
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

func GetExistingReleases(ctx context.Context, tc TrackerConfig) ([]string, error) {
	// Match releases for this repo/commit and find outstanding work for them
	mr := fmt.Sprintf(
		"%v/-/%v/@*/commit/%v",
		tc.Ref.Repo,
		tc.Ref.Filename,
		tc.Commit,
	)
	releasesForCommit, err := tc.Store.Match(ctx, mr)
	if err != nil {
		return nil, fmt.Errorf("failed to match refs: %w", err)
	}

	var out []string
	for _, ref := range releasesForCommit {
		pr, err := refs.Parse(ref)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ref: %w", err)
		}
		pr = pr.SetSubPathType(refs.SubPathTypeNone).
			SetSubPath("").
			SetFragment("")
		out = append(out, pr.String())
	}

	return out, nil
}
