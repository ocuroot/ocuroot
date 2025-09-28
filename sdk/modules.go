package sdk

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/charmbracelet/log"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

type ModuleResolver interface {
	Resolve(module string) (string, []byte, error)
	Child(module string) ModuleResolver
}

type FileSystemResolver struct {
	basePath string
	fs       fs.FS
}

func NewFSResolver(fs fs.FS) ModuleResolver {
	return &FileSystemResolver{
		fs:       fs,
		basePath: "",
	}
}

func NewNullResolver() ModuleResolver {
	return &nullResolver{}
}

type nullResolver struct{}

func (r *nullResolver) Resolve(module string) (string, []byte, error) {
	return "", nil, fmt.Errorf("module %s not found", module)
}

func (r *nullResolver) Child(module string) ModuleResolver {
	return r
}

func (r *FileSystemResolver) Resolve(module string) (string, []byte, error) {
	log.Debug("Resolving module", "module", module, "base", r.basePath)
	var err error
	filename := module
	if r.basePath != "" {
		filename = filepath.Join(r.basePath, filename)
	}
	log.Info("Opening module file", "filename", filename)
	file, err := r.fs.Open(filename)
	if err != nil {
		log.Error("Failed to open module file", "filename", filename, "base", r.basePath, "error", err)
		return "", nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		log.Error("Failed to read module file", "filename", filename, "error", err)
		return "", nil, err
	}
	return filename, data, nil
}

func (r *FileSystemResolver) Child(module string) ModuleResolver {
	newBase := filepath.Join(r.basePath, module)
	newBase = filepath.Dir(newBase)

	log.Debug("Creating child resolver", "module", module, "base", newBase)

	return &FileSystemResolver{
		fs:       r.fs,
		basePath: newBase,
	}
}

func NewModuleLoader(
	resolver ModuleResolver,
	builtinsByVersion map[string]starlark.StringDict,
	previous *moduleLoader,
) *moduleLoader {
	out := &moduleLoader{
		resolver:          resolver,
		builtinsByVersion: builtinsByVersion,
		cache:             map[string]starlark.StringDict{},
		loading:           map[string]struct{}{},
	}

	if previous != nil {
		out.cache = previous.cache
		out.loading = previous.loading
	}
	return out
}

type moduleLoader struct {
	resolver          ModuleResolver
	builtinsByVersion map[string]starlark.StringDict

	cache   map[string]starlark.StringDict
	loading map[string]struct{}
}

func (m *moduleLoader) Load(_ *starlark.Thread, module string) (starlark.StringDict, error) {
	if module, exists := m.cache[module]; exists {
		return module, nil
	}

	if _, exists := m.loading[module]; exists {
		return nil, fmt.Errorf("cycle in load graph")
	}

	m.loading[module] = struct{}{}
	defer delete(m.loading, module)

	log.Debug("Loading module", "module", module)

	filename, data, err := m.resolver.Resolve(module)
	if err != nil {
		return nil, err
	}

	sdkVersion, err := IdentifySDKVersion(filename, data)
	if err != nil {
		return nil, err
	}

	var builtins starlark.StringDict
	if sdkVersion != "" {
		// Try to resolve version alias first
		resolvedVersion := resolveVersionAlias(sdkVersion)
		var exists bool
		builtins, exists = m.builtinsByVersion[resolvedVersion]
		if !exists {
			return nil, fmt.Errorf("version %s not found", sdkVersion)
		}
	}

	loader := NewModuleLoader(m.resolver.Child(module), m.builtinsByVersion, m)
	thread := &starlark.Thread{
		Name: fmt.Sprintf("load %s", module),
		Load: loader.Load,
		Print: func(thread *starlark.Thread, msg string) {
			log.Info(module, "thread", thread.Name, "filename", filename, "msg", msg)
		},
	}

	globals, err := starlark.ExecFileOptions(
		syntax.LegacyFileOptions(),
		thread,
		module,
		string(data),
		builtins,
	)
	if err != nil {
		return nil, err
	}

	m.cache[module] = globals

	return globals, nil
}

func (m *moduleLoader) Cache() map[string]starlark.StringDict {
	return m.cache
}
