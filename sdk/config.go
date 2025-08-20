package sdk

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/sdk/starlarkerrors"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

type Config struct {
	globalFuncs map[string]*starlark.Function

	doWorkFunc *starlark.Function

	Package *Package

	Backend Backend
}

// GlobalFuncs returns the user-defined functions from the loaded configuration
func (c *Config) GlobalFuncs() map[string]*starlark.Function {
	return c.globalFuncs
}

type FunctionContext struct {
	WorkID  WorkID         `json:"work_id"`
	Package *Package       `json:"package,omitempty"`
	Inputs  map[string]any `json:"inputs,omitempty"`
}

type Logger func(log Log)

func LoadConfig(
	ctx context.Context,
	resolver ModuleResolver,
	filename string,
	backend Backend,
	print func(thread *starlark.Thread, msg string),
) (*Config, error) {
	log.Debug("Loading config", "filename", filename)
	c := &configLoader{
		backend:  backend,
		resolver: resolver,
		print:    print,
	}
	out := &Config{
		Backend: backend,
	}
	out.globalFuncs = map[string]*starlark.Function{}

	builtinsByVersion, err := c.LoadBuiltinsForAllVersion(ctx)
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
			// The after function should not be available to the config script
			if m == "after" {
				return false
			}
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
	globals, err := mod.Init(thread, builtins)
	if err != nil {
		return nil, starlarkerrors.Wrap(err)
	}

	if dw, exists := builtins["do_work"]; exists {
		out.doWorkFunc = dw.(*starlark.Function)
	}

	// Add global functions to available globals
	for _, value := range globals {
		if fn, ok := value.(*starlark.Function); ok {
			def := DefForFunction(fn)
			out.globalFuncs[def.String()] = fn
		}
	}
	// Add module functions to available globals
	cache := loader.Cache()
	for _, globals := range cache {
		for _, value := range globals {
			if fn, ok := value.(*starlark.Function); ok {
				def := DefForFunction(fn)
				out.globalFuncs[def.String()] = fn
			}
		}
	}

	// Run the after function
	if after, exists := builtins["after"]; exists {
		if _, err := starlark.Call(thread, after.(*starlark.Function), nil, nil); err != nil {
			return nil, starlarkerrors.Wrap(err)
		}
	}

	out.Package = c.pkg

	return out, nil
}

func (c *Config) Run(
	ctx context.Context,
	f FunctionDef,
	logger Logger,
	functionContext FunctionContext,
) (WorkResult, error) {
	if _, exists := c.globalFuncs[f.String()]; !exists {
		return WorkResult{}, fmt.Errorf("function %s not found", f.String())
	}

	thread := &starlark.Thread{
		Name: string(f.Name),
		Print: c.Backend.WrapPrint(func(thread *starlark.Thread, msg string) {
			cf := thread.CallFrame(1)
			logger(
				Log{
					Timestamp: time.Now(),
					Message:   msg,
					Attributes: map[string]string{
						"thread":   thread.Name,
						"filename": cf.Pos.Filename(),
						"line":     fmt.Sprintf("%d", cf.Pos.Line),
						"col":      fmt.Sprintf("%d", cf.Pos.Col),
					},
				},
			)
		}),
	}

	// Set the context as a local variable so it can be accessed from within the function
	// This can be used for telemetry propagation, for example.
	thread.SetLocal("ctx", ctx)

	gf := c.globalFuncs[f.String()]

	fcJSON, err := json.Marshal(functionContext)
	if err != nil {
		return WorkResult{}, err
	}

	var fcd map[string]interface{}
	if err := json.Unmarshal(fcJSON, &fcd); err != nil {
		return WorkResult{}, err
	}

	params := []starlark.Tuple{}
	for key, value := range fcd {
		valueJSON, err := json.Marshal(value)
		if err != nil {
			return WorkResult{}, err
		}
		params = append(params, starlark.Tuple{starlark.String(key), starlark.String(string(valueJSON))})
	}

	resultValue, err := starlark.Call(
		thread,
		c.doWorkFunc,
		starlark.Tuple{
			gf,
		},
		params,
	)
	if err != nil {
		return WorkResult{
			Err: starlarkerrors.Wrap(err),
		}, nil
	}

	resultString, ok := resultValue.(starlark.String)
	if !ok {
		return WorkResult{
			Err: fmt.Errorf("expected string result, got %T", resultValue),
		}, nil
	}
	var result WorkResult
	if err := json.Unmarshal([]byte(resultString.GoString()), &result); err != nil {
		return WorkResult{
			Err: err,
		}, nil
	}

	return result, nil
}

func IdentifySDKVersion(filename string, data []byte) (string, error) {
	// Parse the source as a first pass to check for the SDK version function
	var ocurootCallCount int
	pf, _, err := starlark.SourceProgramOptions(
		syntax.LegacyFileOptions(),
		filename,
		string(data),
		func(m string) bool {
			if m == "ocuroot" {
				ocurootCallCount++
			}
			return true
		},
	)
	if err != nil {
		return "", err
	}

	if ocurootCallCount == 0 {
		return "", nil
	}

	if ocurootCallCount > 1 {
		return "", fmt.Errorf("multiple ocuroot calls found")
	}

	sdkVersion := ""
	for _, stmt := range pf.Stmts {
		if stmt == nil {
			continue
		}
		if stmt, ok := stmt.(*syntax.ExprStmt); ok {
			if x, ok := stmt.X.(*syntax.CallExpr); ok {
				if fn, ok := x.Fn.(*syntax.Ident); ok && fn.Name == "ocuroot" {
					// Get the SDK version from the first parameter, expecting a string literal
					if len(x.Args) == 1 {
						if arg, ok := x.Args[0].(*syntax.Literal); ok {
							sdkVersion = arg.Value.(string)
						} else {
							return "", fmt.Errorf("ocuroot call must have a string literal as its argument")
						}
					} else {
						return "", fmt.Errorf("ocuroot call must have exactly one argument")
					}
				}
			}
		}
	}

	return sdkVersion, nil
}

type configLoader struct {
	backend  Backend
	resolver ModuleResolver
	pkg      *Package
	print    func(thread *starlark.Thread, msg string)
}

func renderFunction(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		function        *starlark.Function
		requireTopLevel bool
	)
	err := starlark.UnpackArgs("render_function", args, kwargs, "function", &function, "require_top_level?", &requireTopLevel)
	if err != nil {
		return nil, err
	}

	d := starlark.NewDict(4)
	err = d.SetKey(starlark.String("name"), starlark.String(function.Name()))
	if err != nil {
		return nil, err
	}
	posStr := fmt.Sprintf("%v:%v:%v", function.Position().Filename(), function.Position().Line, function.Position().Col)
	if requireTopLevel && function.Position().Col != 1 {
		return nil, fmt.Errorf("function %q must be defined at the top level of a file, it was defined at %s", function.Name(), posStr)
	}

	err = d.SetKey(starlark.String("pos"), starlark.String(posStr))
	if err != nil {
		return nil, err
	}

	w := starlark.NewDict(1)
	err = w.SetKey(starlark.String("function"), d)
	if err != nil {
		return nil, err
	}

	return w, nil
}
