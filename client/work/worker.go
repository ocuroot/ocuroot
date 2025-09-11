package work

import (
	"context"
	"fmt"

	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/client/tui"
)

type Worker struct {
	Tracker release.TrackerConfig

	Tui tui.Tui
}

type GitFilter int

const (
	GitFilterNone GitFilter = iota
	GitFilterCurrentRepoOnly
	GitFilterCurrentCommitOnly
)

type IndentifyWorkRequest struct {
	// Filter work based on the repo and commit required
	GitFilter GitFilter
}

func (w *Worker) IdentifyWork(ctx context.Context, req IndentifyWorkRequest) ([]Work, error) {
	var out []Work

	diffs, err := w.Diff(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}
	out = append(out, diffs...)

	readyRuns, err := w.ReadyRuns(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get ready runs: %w", err)
	}
	out = append(out, readyRuns...)

	reconcilableDeployments, err := w.ReconcilableDeployments(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get reconcilable deployments: %w", err)
	}
	out = append(out, reconcilableDeployments...)

	ops, err := w.Ops(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get ops: %w", err)
	}
	out = append(out, ops...)

	return out, nil
}
