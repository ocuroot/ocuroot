package local

import (
	"context"
	"fmt"
	"os"

	"github.com/ocuroot/ocuroot/lib/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"
	"github.com/oklog/ulid/v2"
)

func CreateTracker(ctx context.Context, repoPath string, ref refs.Ref, commit string, backend sdk.Backend) (*release.ReleaseTracker, error) {
	config, err := ExecutePackage(ctx, repoPath, ref, backend)
	if err != nil {
		return nil, fmt.Errorf("failed to execute package: %w", err)
	}

	if config.Package == nil {
		return nil, fmt.Errorf("package not found")
	}

	releaseState := models.SDKPackageToReleaseSummary(
		models.ReleaseID(
			fmt.Sprintf(
				"local-%s",
				ulid.Make().String(),
			),
		),
		commit,
		config.Package,
	)

	tmpDir, err := os.MkdirTemp("", "ocuroot-local-release")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	// TODO: Cleanup

	refStore, err := refstore.NewFSRefStore(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create ref store: %w", err)
	}

	releaseRef := ref
	releaseRef.ReleaseOrIntent = refs.ReleaseOrIntent{
		Type:  refs.Release,
		Value: string(releaseState.ID),
	}

	if err := refStore.Set(ctx, releaseRef.String(), releaseState); err != nil {
		return nil, fmt.Errorf("failed to set release state: %w", err)
	}

	rt, err := release.NewReleaseTracker(
		config,
		config.Package,
		releaseRef,
		refStore)
	if err != nil {
		return nil, fmt.Errorf("failed to create release tracker: %w", err)
	}

	return rt, nil
}
