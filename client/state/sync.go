package state

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/refs"
)

// Sync identifies all diffs and applies them
func Sync(ctx context.Context, tc release.TrackerConfig) error {
	diffs, err := Diff(ctx, tc.State, tc.Intent)
	if err != nil {
		return fmt.Errorf("failed to get diffs: %w", err)
	}

	log.Info("Syncing", "diffs", diffs)

	for _, diff := range diffs {
		diffRef, err := refs.Parse(diff)
		if err != nil {
			return fmt.Errorf("failed to parse diff ref: %w", err)
		}
		if err := ApplyIntent(ctx, diffRef, tc.State, tc.Intent); err != nil {
			return fmt.Errorf("failed to apply intent (%s): %w", diffRef.String(), err)
		}
	}

	return nil
}
