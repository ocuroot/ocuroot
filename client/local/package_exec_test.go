package local

import (
	"context"
	"testing"

	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/sdk/starlarkerrors"
	"github.com/ocuroot/ocuroot/store/models"
)

func TestPackageExec(t *testing.T) {
	be, _ := NewBackend(
		refs.Ref{
			Filename: ".",
		},
		NewBlockingKVStore[string, string](),
	)

	config, err := ExecutePackage(
		context.Background(),
		"../../tests/minimal/",
		refs.Ref{
			Filename: "basic.ocu.star",
		},
		be,
	)
	if err != nil {
		t.Fatalf("failed to execute package: %v", starlarkerrors.Render(err))
	}

	releaseSummary := models.SDKPackageToReleaseSummary(models.ReleaseID("test"), "commit1", config.Package)
	printJSON(releaseSummary)
}
