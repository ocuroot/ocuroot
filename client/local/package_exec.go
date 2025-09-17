package local

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/sdk"
	"go.starlark.net/starlark"
)

func ExecutePackage(ctx context.Context, root string, ref refs.Ref, backend sdk.Backend) (*sdk.Config, error) {
	return ExecutePackageWithLogging(ctx, root, ref, backend, func(thread *starlark.Thread, msg string) {
		cf := thread.CallFrame(1)
		log.Info(msg, "filename", cf.Pos.Filename(), "line", cf.Pos.Line, "col", cf.Pos.Col)
	})
}

func ExecutePackageWithLogging(ctx context.Context, root string, ref refs.Ref, backend sdk.Backend, logf func(thread *starlark.Thread, msg string)) (*sdk.Config, error) {
	configFile := ref.Filename
	log.Info("Loading config", "root", root, "filename", configFile, "ref", ref)
	config, err := sdk.LoadConfig(
		ctx,
		sdk.NewFSResolver(os.DirFS(root)),
		configFile,
		backend,
		logf,
	)
	if err != nil {
		return nil, fmt.Errorf("LoadConfig: %w", err)
	}

	if config.Package == nil {
		return config, nil
	}

	// Validate all packages in the config
	if validationErrors := config.Package.Validate(); len(validationErrors) > 0 {
		// Format all validation errors into a single error message
		errorMsg := "Package validation failed:\n"
		for _, err := range validationErrors {
			errorMsg += fmt.Sprintf("  - %s\n", err.Error())
		}
		return nil, fmt.Errorf("validation: %s", errorMsg)
	}

	return config, nil
}
