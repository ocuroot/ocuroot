package release

import (
	"context"
	"fmt"

	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
)

func NextReleaseID(ctx context.Context, store refstore.Store, ref refs.Ref) (refs.Ref, error) {
	p, err := refstore.IncrementPath(ctx, store, fmt.Sprintf("%s/-/%s/@r", ref.Repo, ref.Filename))
	if err != nil {
		return refs.Ref{}, fmt.Errorf("failed to increment path: %w", err)
	}
	return refs.Parse(p)
}
