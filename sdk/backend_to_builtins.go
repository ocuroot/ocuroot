package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/refs"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func (c *configLoader) backendToBuiltins(backend Backend) starlark.StringDict {
	out := starlark.StringDict{}

	out["thread"] = c.threadBuiltins()
	out["refs"] = c.refsBuiltins(backend)
	out["repo"] = c.repoBuiltins(backend)
	out["environments"] = c.environmentBuiltins(backend)
	out["packages"] = c.packageBuiltins(backend)
	out["secrets"] = c.secretBuiltins(backend)
	out["http"] = c.httpBuiltins(backend)
	out["host"] = c.hostBuiltins(backend)
	out["store"] = c.storeBuiltins(backend)
	out["debug"] = c.debugBuiltins(backend)

	return out
}

func (c *configLoader) threadBuiltins() starlark.Value {
	threadBuiltins := starlark.StringDict{}
	threadBuiltins["set"] = starlark.NewBuiltin(
		"thread.set",
		func(
			thread *starlark.Thread,
			fn *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {

			var key starlark.String
			var value starlark.Value
			err := starlark.UnpackArgs("thread.set", args, kwargs, "key", &key, "value", &value)
			if err != nil {
				return nil, err
			}
			thread.SetLocal(key.GoString(), value)
			return starlark.None, nil
		})
	threadBuiltins["exists"] = starlark.NewBuiltin(
		"thread.exists", func(
			thread *starlark.Thread,
			fn *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {
			var key starlark.String
			err := starlark.UnpackArgs("thread.exists", args, kwargs, "key", &key)
			if err != nil {
				return nil, err
			}
			return starlark.Bool(thread.Local(key.GoString()) != nil), nil
		})
	threadBuiltins["get"] = starlark.NewBuiltin(
		"thread.get", func(
			thread *starlark.Thread,
			fn *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {
			var key starlark.String
			var defaultValue starlark.Value
			err := starlark.UnpackArgs("thread.get", args, kwargs, "key", &key, "default?", &defaultValue)
			if err != nil {
				return nil, err
			}
			value := thread.Local(key.GoString())
			if value == nil {
				if defaultValue != nil {
					return defaultValue, nil
				}
				return nil, fmt.Errorf("key %s not found", key)
			}
			if sv, ok := value.(starlark.Value); ok {
				return sv, nil
			}
			return nil, fmt.Errorf("key %s is not a starlark.Value", key)
		})

	threadBuiltins["current"] = JSONBuiltin("thread.current", func(ctx context.Context, _ any) (any, error) {
		return ctx, nil
	})
	return starlarkstruct.FromStringDict(starlark.String("thread"), threadBuiltins)
}

func (c *configLoader) refsBuiltins(backend Backend) starlark.Value {
	refsBuiltins := starlark.StringDict{}
	if refsBackend := backend.Refs; refsBackend != nil {
		refsBuiltins["absolute"] = JSONBuiltin("refs.absolute", func(ctx context.Context, ref string) (string, error) {
			refParsed, err := refs.Parse(ref)
			if err != nil {
				return "", fmt.Errorf("failed to parse ref: %w", err)
			}
			absRef, err := refsBackend.Absolute(refParsed)
			if err != nil {
				return "", fmt.Errorf("failed to get absolute ref: %w", err)
			}
			return absRef.String(), nil
		})
	} else {
		refsBuiltins["absolute"] = unimplementedFunction("refs.absolute")
	}
	return starlarkstruct.FromStringDict(starlark.String("refs"), refsBuiltins)
}

func (c *configLoader) repoBuiltins(backend Backend) starlark.Value {
	repoBuiltins := starlark.StringDict{}
	if repoBackend := backend.Repo; repoBackend != nil {
		repoBuiltins["alias"] = JSONBuiltin("repo.alias", func(ctx context.Context, alias string) (any, error) {
			return nil, repoBackend.Alias(ctx, alias)
		})
		repoBuiltins["trigger"] = starlark.NewBuiltin("repo.trigger", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var triggerFN *starlark.Function
			err := starlark.UnpackArgs("repo.trigger", args, kwargs, "fn", &triggerFN)
			if err != nil {
				return nil, err
			}
			repoBackend.Trigger(context.Background(), triggerFN)
			return starlark.None, nil
		})
	} else {
		repoBuiltins["alias"] = unimplementedFunction("repo.alias")
		repoBuiltins["apply"] = unimplementedFunction("repo.apply")
	}
	return starlarkstruct.FromStringDict(starlark.String("repo"), repoBuiltins)
}

func (c *configLoader) environmentBuiltins(backend Backend) starlark.Value {
	environmentBuiltins := starlark.StringDict{}
	if environmentBackend := backend.Environments; environmentBackend != nil {
		environmentBuiltins["all"] = JSONBuiltin("environments.all", func(ctx context.Context, _ any) ([]Environment, error) {
			return environmentBackend.All(ctx)
		})
		environmentBuiltins["register"] = JSONBuiltin("environments.register", func(ctx context.Context, env Environment) (any, error) {
			return nil, environmentBackend.Register(ctx, env)
		})
	} else {
		environmentBuiltins["all"] = unimplementedFunction("environments.all")
		environmentBuiltins["register"] = unimplementedFunction("environments.register")
	}
	return starlarkstruct.FromStringDict(starlark.String("environments"), environmentBuiltins)
}

func (c *configLoader) packageBuiltins(backend Backend) starlark.Value {
	packageBuiltins := starlark.StringDict{}
	if backend.AllowPackageRegistration {
		packageBuiltins["register"] = JSONBuiltin("packages.register", func(ctx context.Context, pkg Package) (any, error) {
			if c.pkg != nil {
				return nil, fmt.Errorf("package already registered")
			}
			c.pkg = &pkg
			return nil, nil
		})
	} else {
		packageBuiltins["register"] = unimplementedFunction("packages.register")
	}
	return starlarkstruct.FromStringDict(starlark.String("packages"), packageBuiltins)
}

func (c *configLoader) httpBuiltins(backend Backend) starlark.Value {
	httpBuiltins := starlark.StringDict{}
	if httpBackend := backend.Http; httpBackend != nil {
		httpBuiltins["get"] = JSONBuiltin("http.get", func(ctx context.Context, req HTTPGetRequest) (HTTPResponse, error) {
			return httpBackend.Get(ctx, req)
		})

		httpBuiltins["post"] = JSONBuiltin("http.post", func(ctx context.Context, req HTTPPostRequest) (HTTPResponse, error) {
			return httpBackend.Post(ctx, req)
		})
	} else {
		httpBuiltins["get"] = unimplementedFunction("http.get")
		httpBuiltins["post"] = unimplementedFunction("http.post")
	}
	return starlarkstruct.FromStringDict(starlark.String("http"), httpBuiltins)
}

func (c *configLoader) secretBuiltins(backend Backend) starlark.Value {
	secretsBuiltins := starlark.StringDict{}
	if secretsBackend := backend.Secrets; secretsBackend != nil {
		secretsBuiltins["secret"] = JSONBuiltin("secrets.secret", func(ctx context.Context, value string) (any, error) {
			secretsBackend.Register(value)
			return nil, nil
		})
	} else {
		secretsBuiltins["secret"] = unimplementedFunction("secrets.secret")
	}
	return starlarkstruct.FromStringDict(starlark.String("secrets"), secretsBuiltins)
}

// Define a function type that matches the Write method signature
type CustomWriter func(p []byte) (n int, err error)

// Implement the Write method for MyWriterFunc
func (cw CustomWriter) Write(p []byte) (n int, err error) {
	return cw(p)
}

func (c *configLoader) hostBuiltins(backend Backend) starlark.Value {
	hostBuiltins := starlark.StringDict{}
	if hostBackend := backend.Host; hostBackend != nil {
		hostBuiltins["os"] = JSONBuiltin("host.os", func(_ context.Context, _ any) (string, error) {
			return hostBackend.OS(), nil
		})
		hostBuiltins["arch"] = JSONBuiltin("host.arch", func(_ context.Context, _ any) (string, error) {
			return hostBackend.Arch(), nil
		})
		hostBuiltins["env"] = JSONBuiltin("host.env", func(_ context.Context, _ any) (map[string]string, error) {
			return hostBackend.Env(), nil
		})
		hostBuiltins["shell"] = JSONBuiltinWithThread("host.shell", func(thread *starlark.Thread, req HostShellRequest) (HostShellResponse, error) {
			var output io.Writer = CustomWriter(func(p []byte) (n int, err error) {

				lines := strings.Split(strings.TrimRight(string(p), "\n"), "\n")
				for _, line := range lines {
					thread.Print(thread, line)
				}
				return len(p), nil
			})
			if req.Mute {
				output = io.Discard
			}
			return hostBackend.Shell(contextFromThread(thread), req, output)
		})
		hostBuiltins["working_dir"] = JSONBuiltin("host.working_dir", func(_ context.Context, _ any) (string, error) {
			return hostBackend.WorkingDir(), nil
		})
		hostBuiltins["read_file"] = JSONBuiltin("host.read_file", func(_ context.Context, path string) (string, error) {
			return hostBackend.ReadFile(context.Background(), path)
		})
		hostBuiltins["write_file"] = JSONBuiltin("host.write_file", func(_ context.Context, req WriteFileRequest) (any, error) {
			return nil, hostBackend.WriteFile(context.Background(), req)
		})
		hostBuiltins["read_dir"] = JSONBuiltin("host.read_dir", func(_ context.Context, path string) ([]string, error) {
			return hostBackend.ReadDir(context.Background(), path)
		})
		hostBuiltins["is_dir"] = JSONBuiltin("host.is_dir", func(_ context.Context, path string) (bool, error) {
			return hostBackend.IsDir(context.Background(), path)
		})
	} else {
		hostBuiltins["os"] = unimplementedFunction("host.os")
		hostBuiltins["arch"] = unimplementedFunction("host.arch")
		hostBuiltins["env"] = unimplementedFunction("host.env")
		hostBuiltins["shell"] = unimplementedFunction("host.shell")
		hostBuiltins["working_dir"] = unimplementedFunction("host.working_dir")
		hostBuiltins["read_file"] = unimplementedFunction("host.read_file")
		hostBuiltins["write_file"] = unimplementedFunction("host.write_file")
		hostBuiltins["read_dir"] = unimplementedFunction("host.read_dir")
		hostBuiltins["is_dir"] = unimplementedFunction("host.is_dir")
	}
	return starlarkstruct.FromStringDict(starlark.String("host"), hostBuiltins)
}

func (c *configLoader) storeBuiltins(backend Backend) starlark.Value {
	storeBuiltins := starlark.StringDict{}
	if storeBackend := backend.Store; storeBackend != nil {
		storeBuiltins["set"] = JSONBuiltin("store.set", func(ctx context.Context, store Store) (any, error) {
			err := storeBackend.Set(store)
			return nil, err
		})
	} else {
		storeBuiltins["set"] = unimplementedFunction("store.set")
	}
	return starlarkstruct.FromStringDict(starlark.String("store"), storeBuiltins)
}

func getDebugFrame(thread *starlark.Thread) DebugFrame {
	frm := thread.DebugFrame(2)
	locals := make([]DebugBinding, frm.NumLocals())
	for i := 0; i < frm.NumLocals(); i++ {
		binding, val := frm.Local(i)
		valStr := ""
		valType := "none"
		if val != nil {
			valStr = val.String()
			valType = fmt.Sprintf("%T", val)
		}
		locals[i] = DebugBinding{
			Name: binding.Name,
			Pos:  binding.Pos.String(),
			Type: valType,
			Val:  valStr,
		}
	}
	return DebugFrame{
		Locals: locals,
	}
}

func (c *configLoader) debugBuiltins(backend Backend) starlark.Value {
	debugBuiltins := starlark.StringDict{}
	if debugBackend := backend.Debug; debugBackend != nil {
		debugBuiltins["brk"] = starlark.NewBuiltin("debug.brk", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			debugBackend.Break(getDebugFrame(thread))
			return starlark.None, nil
		})
		debugBuiltins["emit"] = starlark.NewBuiltin("debug.emit", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			frm := getDebugFrame(thread)
			frmJSON, err := json.MarshalIndent(frm, "", "  ")
			if err != nil {
				log.Error("failed to marshal frame", "error", err)
				frmJSON = []byte("<failed to marshal>")
			}
			thread.Print(thread, string(frmJSON))
			return starlark.None, nil
		})
	} else {
		debugBuiltins["brk"] = unimplementedFunction("debug.brk")
		debugBuiltins["emit"] = unimplementedFunction("debug.emit")
	}
	return starlarkstruct.FromStringDict(starlark.String("debug"), debugBuiltins)
}

func contextFromThread(thread *starlark.Thread) context.Context {
	ctx := thread.Local("ctx")
	if ctx == nil {
		return context.Background()
	}
	return ctx.(context.Context)
}

func JSONBuiltinWithThread[T any, R any](name string, callback func(*starlark.Thread, T) (R, error)) starlark.Value {
	return starlark.NewBuiltin(name, func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var j string
		err := starlark.UnpackArgs(name, args, kwargs, "json?", &j)
		if err != nil {
			return starlark.None, fmt.Errorf("builtin: failed to unpack arguments: %w", err)
		}

		var out T
		if j != "" {
			if err := json.Unmarshal([]byte(j), &out); err != nil {
				return starlark.None, fmt.Errorf("builtin: failed to unmarshal JSON: %w", err)
			}
		}

		res, err := callback(thread, out)
		if err != nil {
			return starlark.None, err
		}

		resJSON, err := json.Marshal(res)
		if err != nil {
			return starlark.None, fmt.Errorf("builtin: failed to marshal result: %w", err)
		}

		return starlark.String(string(resJSON)), nil
	})
}

func JSONBuiltin[T any, R any](name string, callback func(context.Context, T) (R, error)) starlark.Value {
	return starlark.NewBuiltin(name, func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var j string
		err := starlark.UnpackArgs(name, args, kwargs, "json?", &j)
		if err != nil {
			return starlark.None, fmt.Errorf("%v> failed to unpack arguments: %w", name, err)
		}

		var out T
		if j != "" {
			if err := json.Unmarshal([]byte(j), &out); err != nil {
				return starlark.None, fmt.Errorf("%v> failed to unmarshal JSON: %w", name, err)
			}
		}

		res, err := callback(contextFromThread(thread), out)
		if err != nil {
			return starlark.None, fmt.Errorf("%v> failed to execute callback: %w", name, err)
		}

		resJSON, err := json.Marshal(res)
		if err != nil {
			return starlark.None, fmt.Errorf("%v> failed to marshal result: %w", name, err)
		}

		return starlark.String(string(resJSON)), nil
	})
}

func unimplementedFunction(name string) *starlark.Builtin {
	return starlark.NewBuiltin(name, func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, fmt.Errorf("%s is not available under the current configuration", name)
	})
}
