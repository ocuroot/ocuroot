package local

import (
	"context"
	"testing"

	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/sdk/starlarkerrors"
)

func TestPackageExec(t *testing.T) {
	be, _ := NewBackend(
		refs.Ref{
			Filename: ".",
		},
	)

	_, err := ExecutePackage(
		context.Background(),
		"../../tests/minimal/",
		refs.Ref{
			Filename: "basic.ocu.star",
		},
		be,
	)
	if err != nil {
		t.Fatalf("failed to load config: %v", starlarkerrors.Render(err))
	}
}
