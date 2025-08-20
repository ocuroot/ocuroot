package local

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/sdk"
	"go.starlark.net/starlark"
)

func ExecutePackage(ctx context.Context, root string, ref refs.Ref, backend sdk.Backend) (*sdk.Config, error) {
	configFile := ref.Filename
	log.Info("Loading config", "root", root, "filename", configFile, "ref", ref)
	config, err := sdk.LoadConfig(
		ctx,
		sdk.NewFSResolver(os.DirFS(root)),
		configFile,
		backend,
		func(thread *starlark.Thread, msg string) {
			cf := thread.CallFrame(1)
			log.Info(msg, "filename", cf.Pos.Filename(), "line", cf.Pos.Line, "col", cf.Pos.Col)
		},
	)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("validation failed: %s", errorMsg)
	}

	return config, nil
}

func printJSON(arg ...any) {
	for _, v := range arg {
		j, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(string(j))
	}
}
