package sdk

import (
	"context"
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"
)

// EvalWithGlobals executes expr with additional globals and returns both the result and final globals
// This is useful for REPL scenarios where you want to capture user-defined functions
func EvalWithGlobals(ctx context.Context, backend Backend, sdkVersion string, expr string, additionalGlobals starlark.StringDict) (any, starlark.StringDict, error) {
	c := &configLoader{
		backend: backend,
	}
	builtinsByVersion, err := c.LoadBuiltinsForAllVersion(ctx)
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

	// Merge SDK builtins with additional globals
	mergedGlobals := make(starlark.StringDict)
	for k, v := range builtins {
		mergedGlobals[k] = v
	}
	for k, v := range additionalGlobals {
		mergedGlobals[k] = v
	}

	thread := &starlark.Thread{
		Name: "eval",
		Print: func(thread *starlark.Thread, msg string) {
			// Default print function to avoid nil dereference
			fmt.Println(msg)
		},
	}

	opts := syntax.FileOptions{}
	opts.LoadBindsGlobally = true

	// parse
	wasRead := false
	f, err := opts.ParseCompoundStmt("<stdin>", func() ([]byte, error) {
		if !wasRead {
			wasRead = true
			return []byte(expr + "\n"), nil
		}
		return nil, nil
	})
	if err != nil {
		return nil, nil, err
	}

	se := soleExpr(f)
	if se == nil {
		return nil, nil, fmt.Errorf("no expression found")
	}

	r, err := starlark.EvalExprOptions(&opts, thread, se, mergedGlobals)
	if err != nil {
		return nil, nil, err
	}

	return convertValue(r), mergedGlobals, nil
}

// Eval executes expr and returns the result
func Eval(ctx context.Context, backend Backend, sdkVersion string, expr string) (any, error) {
	c := &configLoader{
		backend: backend,
	}
	builtinsByVersion, err := c.LoadBuiltinsForAllVersion(ctx)
	if err != nil {
		return nil, err
	}

	var builtins starlark.StringDict
	if sdkVersion != "" {
		// Try to resolve version alias first
		resolvedVersion := resolveVersionAlias(sdkVersion)
		var exists bool
		builtins, exists = builtinsByVersion[resolvedVersion]
		if !exists {
			return nil, fmt.Errorf("version %s not found", sdkVersion)
		}
	}
	thread := &starlark.Thread{
		Name: "eval",
		Print: func(thread *starlark.Thread, msg string) {
			// Default print function to avoid nil dereference
			fmt.Println(msg)
		},
	}

	opts := syntax.FileOptions{}
	opts.LoadBindsGlobally = true

	// parse
	wasRead := false
	f, err := opts.ParseCompoundStmt("<stdin>", func() ([]byte, error) {
		if !wasRead {
			wasRead = true
			return []byte(expr + "\n"), nil
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}

	se := soleExpr(f)
	if se == nil {
		return nil, fmt.Errorf("no expression found")
	}

	r, err := starlark.EvalExprOptions(&opts, thread, se, builtins)
	if err != nil {
		return nil, err
	}

	return convertValue(r), nil
}

func convertValue(value starlark.Value) any {
	switch value := value.(type) {
	case starlark.NoneType:
		return nil
	case starlark.Bool:
		return bool(value)
	case starlark.Int:
		out, _ := value.Int64()
		return out
	case starlark.Float:
		return float64(value)
	case starlark.String:
		return string(value)
	case *starlark.List:
		out := make([]any, value.Len())
		for i := 0; i < value.Len(); i++ {
			out[i] = convertValue(value.Index(i))
		}
		return out
	case *starlark.Dict:
		out := make(map[any]any)
		items := value.Items()
		for _, item := range items {
			key, val := item[0], item[1]
			out[convertValue(key)] = convertValue(val)
		}
		return out
	case *starlarkstruct.Struct:
		out := make(map[any]any)
		for _, attr := range value.AttrNames() {
			a, err := value.Attr(attr)
			if err != nil {
				return fmt.Errorf("error getting attribute %s: %w", attr, err)
			}
			out[attr] = convertValue(a)
		}
		return out
	default:
		return fmt.Sprintf("%T:%v", value, value)
	}
}

func soleExpr(f *syntax.File) syntax.Expr {
	if len(f.Stmts) == 1 {
		if stmt, ok := f.Stmts[0].(*syntax.ExprStmt); ok {
			return stmt.X
		}
	}
	return nil
}
