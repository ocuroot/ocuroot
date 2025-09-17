package commands

import (
	"context"
	"fmt"

	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
)

type RepoCommitTuple struct {
	Repo   string
	Commit string
}

func releasesForCommit(ctx context.Context, state refstore.Store, repo string, commit string) ([]refs.Ref, error) {
	// Match releases for this repo/commit and find outstanding runs for them
	mr := fmt.Sprintf(
		"%v/-/**/@*/commit/%v",
		repo,
		commit,
	)
	releasesForCommit, err := state.Match(ctx, mr)
	if err != nil {
		return nil, fmt.Errorf("failed to match refs: %w", err)
	}

	var out []refs.Ref
	for _, ref := range releasesForCommit {
		pr, err := refs.Parse(ref)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ref: %w", err)
		}
		pr = pr.SetSubPathType(refs.SubPathTypeNone).
			SetSubPath("").
			SetFragment("")
		out = append(out, pr)
	}

	return out, nil
}
