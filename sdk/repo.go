package sdk

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/log"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

func LoadRepo(
	resolver ModuleResolver,
	filename string,
	backend Backend,
	print func(thread *starlark.Thread, msg string),
) ([]byte, error) {
	log.Debug("Loading repo", "filename", filename)
	c := &configLoader{
		backend:  backend,
		resolver: resolver,
	}

	builtinsByVersion, err := c.LoadBuiltinsForAllVersion()
	if err != nil {
		return nil, err
	}

	filename, data, err := resolver.Resolve(filename)
	if err != nil {
		return nil, err
	}

	// First pass to check for the SDK version function
	sdkVersion, err := IdentifySDKVersion(filename, data)
	if err != nil {
		return nil, err
	}

	var builtins starlark.StringDict
	if sdkVersion != "" {
		var exists bool
		builtins, exists = builtinsByVersion[sdkVersion]
		if !exists {
			return nil, fmt.Errorf("version %s not found", sdkVersion)
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
		return nil, err
	}

	loader := NewModuleLoader(resolver.Child(filename), builtinsByVersion, nil)

	thread := &starlark.Thread{
		Name:  filename,
		Load:  loader.Load,
		Print: backend.WrapPrint(print),
	}
	_, err = mod.Init(thread, builtins)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func LoadRepoFromBytes(
	resolver ModuleResolver,
	filename string,
	data []byte,
	backend Backend,
	print func(thread *starlark.Thread, msg string),
) ([]byte, error) {
	log.Debug("Loading repo", "filename", filename)
	c := &configLoader{
		backend:  backend,
		resolver: resolver,
	}

	builtinsByVersion, err := c.LoadBuiltinsForAllVersion()
	if err != nil {
		return nil, err
	}

	// First pass to check for the SDK version function
	sdkVersion, err := IdentifySDKVersion(filename, data)
	if err != nil {
		return nil, err
	}

	var builtins starlark.StringDict
	if sdkVersion != "" {
		var exists bool
		builtins, exists = builtinsByVersion[sdkVersion]
		if !exists {
			return nil, fmt.Errorf("version %s not found", sdkVersion)
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
		return nil, err
	}

	loader := NewModuleLoader(resolver.Child(filename), builtinsByVersion, nil)

	thread := &starlark.Thread{
		Name:  filename,
		Load:  loader.Load,
		Print: backend.WrapPrint(print),
	}
	_, err = mod.Init(thread, builtins)
	if err != nil {
		return nil, err
	}

	return data, nil
}
