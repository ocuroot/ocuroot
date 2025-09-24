package sdk

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/about"
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

// resolveVersionAlias resolves a version to its target SDK version using semver constraints
// Supports patterns like "0.3.x", ">=0.3", "0.3.14", etc.
func resolveVersionAlias(version string) string {
	// Handle special semver patterns including wildcards and constraints
	if strings.Contains(version, "x") || strings.Contains(version, "X") || strings.Contains(version, "*") || 
	   strings.Contains(version, ">=") || strings.Contains(version, "<=") || strings.Contains(version, ">") || 
	   strings.Contains(version, "<") || strings.Contains(version, "~") || strings.Contains(version, "^") ||
	   strings.Contains(version, "-") {
		return resolveVersionConstraint(version)
	}
	
	// Handle exact version matching - check if it's a 0.3.x version (where x is any number)
	if strings.HasPrefix(version, "0.3.") && version != "0.3.0" {
		parts := strings.Split(version, ".")
		if len(parts) == 3 {
			// Validate that the patch version is numeric
			if _, err := fmt.Sscanf(parts[2], "%d", new(int)); err == nil {
				return "0.3.0"
			}
		}
	}
	
	// Return the original version if no alias found
	return version
}

// resolveVersionConstraint resolves semver constraints to the appropriate SDK version
func resolveVersionConstraint(constraint string) string {
	// Get available SDK versions
	availableVersions := AvailableVersions()
	
	// Handle semver constraints including wildcards (x, X, *), ranges (>=, ~, ^), etc.
	// The semver library natively supports wildcards, so no custom handling needed
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		// If constraint parsing fails, return original version
		return constraint
	}
	
	// Find the best matching version
	var bestMatch *semver.Version
	for _, v := range availableVersions {
		version, err := semver.NewVersion(v)
		if err != nil {
			continue
		}
		
		if c.Check(version) {
			if bestMatch == nil || version.GreaterThan(bestMatch) {
				bestMatch = version
			}
		}
	}
	
	if bestMatch != nil {
		return bestMatch.String()
	}
	
	// Return original constraint if no match found
	return constraint
}

// getCurrentSDKVersion returns the appropriate SDK version based on the current binary version
func getCurrentSDKVersion() string {
	binaryVersion := about.Version
	if binaryVersion == "dev" {
		return "0.3.0" // Default for development
	}
	
	// Extract major.minor from binary version
	parts := strings.Split(binaryVersion, ".")
	if len(parts) >= 2 {
		majorMinor := fmt.Sprintf("%s.%s", parts[0], parts[1])
		// Map to appropriate SDK version
		switch majorMinor {
		case "0.3":
			return "0.3.0"
		default:
			return "0.3.0" // Fallback to current SDK version
		}
	}
	
	return "0.3.0" // Fallback
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

func (c *configLoader) LoadBuiltinsForAllVersion(ctx context.Context) (map[string]starlark.StringDict, error) {
	// Execute all the builtins
	files, err := Builtins.ReadDir("sdk")
	if err != nil {
		return nil, err
	}

	builtinsByVersion := map[string]starlark.StringDict{}
	for _, f := range files {
		if f.IsDir() {
			version := strings.ReplaceAll(f.Name(), "_", ".")
			builtins, err := c.LoadBuiltins(ctx, version)
			if err != nil {
				return nil, err
			}
			builtinsByVersion[version] = builtins
		}
	}

	// Add version aliases to support binary versions mapping to SDK versions
	// We need to check all possible 0.3.x versions that might be requested
	// Since we can't predict all possible patch versions, we'll handle this dynamically
	// in the version resolution logic instead of pre-populating aliases

	return builtinsByVersion, nil
}

func (c *configLoader) LoadBuiltins(ctx context.Context, version string) (starlark.StringDict, error) {
	log.Info("Loading builtins", "version", version)

	backendBuiltins := c.backendToBuiltins(ctx, c.backend)

	defaultBuiltins := starlark.StringDict{
		"struct":          starlark.NewBuiltin("struct", starlarkstruct.Make),
		"render_function": starlark.NewBuiltin("render_function", renderFunction),
		"json":            starlarkjson.Module,
		"backend":         starlarkstruct.FromStringDict(starlark.String("backend"), backendBuiltins),
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
