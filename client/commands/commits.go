package commands

import (
	"context"
	"fmt"
	"path"
	"strings"

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

func getRepoAndCommitForRelease(ctx context.Context, ref string, state refstore.Store) (RepoCommitTuple, error) {
	pr, err := refs.Parse(ref)
	if err != nil {
		return RepoCommitTuple{}, fmt.Errorf("failed to parse ref: %w", err)
	}
	pr = pr.SetSubPathType(refs.SubPathTypeNone).
		SetSubPath("").
		SetFragment("")

	if strings.Contains(string(pr.Release), ".") {
		return RepoCommitTuple{
			Repo:   pr.Repo,
			Commit: strings.Split(string(pr.Release), ".")[0],
		}, nil
	}

	// Use the commit marker to identify the commit
	commitGlob := fmt.Sprintf("%v/commit/*", pr.SetSubPathType(refs.SubPathTypeNone).SetSubPath("").SetFragment(""))
	commits, err := state.Match(ctx, commitGlob)
	if err != nil {
		return RepoCommitTuple{}, fmt.Errorf("failed to match commits: %w", err)
	}
	if len(commits) == 0 {
		return RepoCommitTuple{}, fmt.Errorf("no commits found for %v", pr)
	}
	return RepoCommitTuple{
		Repo:   pr.Repo,
		Commit: path.Base(commits[0]),
	}, nil
}
