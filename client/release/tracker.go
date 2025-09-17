package release

import (
	"context"
	"fmt"

	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
)

type TrackerConfig struct {
	Commit   string
	RepoPath string
	Ref      refs.Ref

	Intent refstore.Store
	State  refstore.Store

	StoreConfig *sdk.Store
}

func GetExistingReleases(ctx context.Context, tc TrackerConfig) ([]string, error) {
	// Match releases for this repo/commit and find outstanding runs for them
	mr := fmt.Sprintf(
		"%v/-/%v/@*/commit/%v",
		tc.Ref.Repo,
		tc.Ref.Filename,
		tc.Commit,
	)
	releasesForCommit, err := tc.State.Match(ctx, mr)
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
