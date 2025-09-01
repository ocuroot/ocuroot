package sdk

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

func TestGetArgs(t *testing.T) {
	prog := `
def foo(a, b):
	return a, b
def bar(a, b=1):
	return a, b

fooArgs = get_args(foo)
barArgs = get_args(bar)
	`

	thread := &starlark.Thread{
		Name: "test",
	}
	globals, err := starlark.ExecFileOptions(
		syntax.LegacyFileOptions(),
		thread,
		"test.star",
		prog,
		starlark.StringDict{
			"get_args": starlark.NewBuiltin("get_args", getArgs),
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	fooArgsJSON := globals["fooArgs"].(starlark.String)
	barArgsJSON := globals["barArgs"].(starlark.String)

	var fooArgs getArgsOut
	var barArgs getArgsOut

	if err := json.Unmarshal([]byte(fooArgsJSON.GoString()), &fooArgs); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(barArgsJSON.GoString()), &barArgs); err != nil {
		t.Fatal(err)
	}

	fooArgsExpected := getArgsOut{
		Args:   []string{"a", "b"},
		KWArgs: []string{},
	}

	barArgsExpected := getArgsOut{
		Args:   []string{"a"},
		KWArgs: []string{"b"},
	}

	if !cmp.Equal(fooArgs, fooArgsExpected) {
		t.Fatalf("expected fooArgs to be %+v, got %+v", fooArgsExpected, fooArgs)
	}

	if !cmp.Equal(barArgs, barArgsExpected) {
		t.Fatalf("expected barArgs to be %+v, got %+v", barArgsExpected, barArgs)
	}
}
