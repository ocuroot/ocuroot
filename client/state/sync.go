package state

import (
	"context"
	"fmt"

	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
)

// Sync identifies all diffs and applies them
func Sync(ctx context.Context, store refstore.Store) error {
	diffs, err := Diff(ctx, store)
	if err != nil {
		return fmt.Errorf("failed to get diffs: %w", err)
	}

	for _, diff := range diffs {
		diffRef, err := refs.Parse(diff)
		if err != nil {
			return fmt.Errorf("failed to parse diff ref: %w", err)
		}
		if err := ApplyIntent(ctx, diffRef, store); err != nil {
			return fmt.Errorf("failed to apply intent (%s): %w", diffRef.String(), err)
		}
	}

	return nil
}
