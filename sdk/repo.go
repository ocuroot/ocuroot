package sdk

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/log"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

func LoadRepo(
	ctx context.Context,
	resolver ModuleResolver,
	filename string,
	backend Backend,
	print func(thread *starlark.Thread, msg string),
) (starlark.StringDict, []byte, error) {
	log.Debug("Loading repo", "filename", filename)
	c := &configLoader{
		backend:  backend,
		resolver: resolver,
	}

	builtinsByVersion, err := c.LoadBuiltinsForAllVersion(ctx)
	if err != nil {
		return nil, nil, err
	}

	filename, data, err := resolver.Resolve(filename)
	if err != nil {
		return nil, nil, err
	}

	// First pass to check for the SDK version function
	sdkVersion, err := IdentifySDKVersion(filename, data)
	if err != nil {
		return nil, nil, err
	}

	var builtins starlark.StringDict
	if sdkVersion != "" {
		// Try to resolve version alias first
		resolvedVersion := resolveVersionAlias(sdkVersion)
		var exists bool
		builtins, exists = builtinsByVersion[resolvedVersion]
		if !exists {
			return nil, nil, fmt.Errorf("version %s not found", sdkVersion)
		}
	}

	_, mod, err := starlark.SourceProgramOptions(
		syntax.LegacyFileOptions(),
		filepath.Base(filename),
		string(data),
		func(m string) bool {
			return builtins.Has(m)
		},
	)
	if err != nil {
		return nil, nil, err
	}

	loader := NewModuleLoader(resolver.Child(filename), builtinsByVersion, nil)

	thread := &starlark.Thread{
		Name:  filename,
		Load:  loader.Load,
		Print: backend.WrapPrint(print),
	}
	globals, err := mod.Init(thread, builtins)
	if err != nil {
		return nil, nil, err
	}

	return globals, data, nil
}

func LoadRepoFromBytes(
	ctx context.Context,
	resolver ModuleResolver,
	filename string,
	data []byte,
	backend Backend,
	print func(thread *starlark.Thread, msg string),
) (starlark.StringDict, []byte, error) {
	log.Debug("Loading repo", "filename", filename)
	c := &configLoader{
		backend:  backend,
		resolver: resolver,
	}

	builtinsByVersion, err := c.LoadBuiltinsForAllVersion(ctx)
	if err != nil {
		return nil, nil, err
	}

	// First pass to check for the SDK version function
	sdkVersion, err := IdentifySDKVersion(filename, data)
	if err != nil {
		return nil, nil, err
	}

	var builtins starlark.StringDict
	if sdkVersion != "" {
		// Try to resolve version alias first
		resolvedVersion := resolveVersionAlias(sdkVersion)
		var exists bool
		builtins, exists = builtinsByVersion[resolvedVersion]
		if !exists {
			return nil, nil, fmt.Errorf("version %s not found", sdkVersion)
		}
	}

	_, mod, err := starlark.SourceProgramOptions(
		syntax.LegacyFileOptions(),
		filepath.Base(filename),
		string(data),
		func(m string) bool {
			return builtins.Has(m)
		},
	)
	if err != nil {
		return nil, nil, err
	}

	loader := NewModuleLoader(resolver.Child(filename), builtinsByVersion, nil)

	thread := &starlark.Thread{
		Name:  filename,
		Load:  loader.Load,
		Print: backend.WrapPrint(print),
	}
	globals, err := mod.Init(thread, builtins)
	if err != nil {
		return nil, nil, err
	}

	return globals, data, nil
}
