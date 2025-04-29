package sdk

import (
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	starlarkjson "go.starlark.net/lib/json"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"
)

//go:embed sdk
var Builtins embed.FS

func AvailableVersions() []string {
	files, err := Builtins.ReadDir("sdk")
	if err != nil {
		return nil
	}

	var versions []string
	for _, f := range files {
		if f.IsDir() {
			versions = append(versions, f.Name())
		}
	}

	sort.Strings(versions)

	return versions
}

func GetVersionStubs(version string) map[string]string {
	var out map[string]string = map[string]string{}

	files, err := Builtins.ReadDir(fmt.Sprintf("sdk/%s", version))
	if err != nil {
		log.Error("Failed to read builtin files", "version", version, "err", err)
		return nil
	}

	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".star") {
			continue
		}
		data, err := Builtins.ReadFile(fmt.Sprintf("sdk/%s/%s", version, f.Name()))
		if err != nil {
			log.Error("Failed to read builtin", "version", version, "file", f.Name(), "err", err)
			continue
		}
		out[f.Name()] = string(data)
	}

	return out
}

func (c *configLoader) LoadBuiltinsForAllVersion() (map[string]starlark.StringDict, error) {
	// Execute all the builtins
	files, err := Builtins.ReadDir("sdk")
	if err != nil {
		return nil, err
	}

	builtinsByVersion := map[string]starlark.StringDict{}
	for _, f := range files {
		if f.IsDir() {
			version := strings.ReplaceAll(f.Name(), "_", ".")
			builtins, err := c.LoadBuiltins(version)
			if err != nil {
				return nil, err
			}
			builtinsByVersion[version] = builtins
		}
	}

	return builtinsByVersion, nil
}

func (c *configLoader) LoadBuiltins(version string) (starlark.StringDict, error) {
	log.Info("Loading builtins", "version", version)

	backendBuiltins := c.backendToBuiltins(c.backend)

	defaultBuiltins := starlark.StringDict{
		"struct":            starlark.NewBuiltin("struct", starlarkstruct.Make),
		"render_function":   starlark.NewBuiltin("render_function", renderFunction),
		"json":              starlarkjson.Module,
		"get_handoff_graph": starlark.NewBuiltin("get_handoff_graph", c.getHandoffGraph),
		"backend":           starlarkstruct.FromStringDict(starlark.String("backend"), backendBuiltins),
	}

	// Execute all the builtins
	files, err := Builtins.ReadDir(fmt.Sprintf("sdk/%s", version))
	if err != nil {
		return nil, err
	}

	var builtInsByFile map[string]starlark.StringDict = map[string]starlark.StringDict{}
	var loading map[string]struct{} = map[string]struct{}{}

	var loadFromFile func(_ *starlark.Thread, module string) (starlark.StringDict, error)
	loadFromFile = func(_ *starlark.Thread, module string) (starlark.StringDict, error) {
		if module, exists := builtInsByFile[module]; exists {
			return module, nil
		}

		if _, exists := loading[module]; exists {
			log.Error("Cycle in load graph", "module", module)
			return nil, fmt.Errorf("cycle in load graph")
		}

		loading[module] = struct{}{}
		defer delete(loading, module)

		log.Debug("Loading module", "module", module)
		data, err := Builtins.ReadFile(fmt.Sprintf("sdk/%s/%s", version, module))
		if err != nil {
			return nil, err
		}

		thread := &starlark.Thread{
			Name: "builtin",
			Load: loadFromFile,
			Print: func(thread *starlark.Thread, msg string) {
				log.Info("builtin", "thread", thread.Name, "msg", msg)
			},
		}

		globals, err := starlark.ExecFileOptions(
			syntax.LegacyFileOptions(),
			thread,
			fmt.Sprintf("sdk/%s/%s", version, module),
			string(data),
			defaultBuiltins,
		)
		if err != nil {
			return nil, err
		}

		builtInsByFile[module] = globals

		return globals, nil
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".star") {
			continue
		}

		_, err := loadFromFile(nil, file.Name())
		if err != nil {
			return nil, err
		}
	}

	var out starlark.StringDict = starlark.StringDict{}
	for _, globals := range builtInsByFile {
		for k, v := range globals {
			if strings.HasPrefix(k, "_") {
				continue
			}
			if _, ok := out[k]; ok {
				return nil, fmt.Errorf("duplicate global %s", k)
			}
			out[k] = v
		}
	}

	// Provide a function for the version selector
	out["ocuroot"] = starlark.NewBuiltin("ocuroot", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})

	// Make sure we have struct and json
	out["struct"] = starlark.NewBuiltin("struct", starlarkstruct.Make)
	out["json"] = starlarkjson.Module

	return out, nil
}
