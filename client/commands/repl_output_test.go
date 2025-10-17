package commands

import (
	"testing"

	"go.starlark.net/starlark"
)

func TestRenderValue_Primitives(t *testing.T) {
	tests := []struct {
		name     string
		value    starlark.Value
		expected string
	}{
		{"None", starlark.None, "None"},
		{"Bool true", starlark.True, "True"},
		{"Bool false", starlark.False, "False"},
		{"Int", starlark.MakeInt(42), "42"},
		{"Float", starlark.Float(3.14), "3.14"},
		{"String", starlark.String("hello"), `"hello"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderValue(tt.value)
			if result != tt.expected {
				t.Errorf("RenderValue() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRenderValue_EmptyCollections(t *testing.T) {
	tests := []struct {
		name     string
		value    starlark.Value
		expected string
	}{
		{"Empty list", starlark.NewList(nil), "[]"},
		{"Empty tuple", starlark.Tuple{}, "()"},
		{"Empty dict", starlark.NewDict(0), "{}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderValue(tt.value)
			if result != tt.expected {
				t.Errorf("RenderValue() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRenderValue_List(t *testing.T) {
	list := starlark.NewList([]starlark.Value{
		starlark.MakeInt(1),
		starlark.MakeInt(2),
		starlark.MakeInt(3),
	})

	result := RenderValue(list)
	
	// Check that it contains tree characters and values
	if !contains(result, "├─") && !contains(result, "└─") {
		t.Errorf("Expected tree characters in list output, got: %v", result)
	}
	if !contains(result, "1") || !contains(result, "2") || !contains(result, "3") {
		t.Errorf("Expected list values in output, got: %v", result)
	}
}

func TestRenderValue_Dict(t *testing.T) {
	dict := starlark.NewDict(2)
	dict.SetKey(starlark.String("key1"), starlark.MakeInt(100))
	dict.SetKey(starlark.String("key2"), starlark.String("value"))

	result := RenderValue(dict)
	
	// Check that it contains tree characters and key-value pairs
	if !contains(result, "├─") && !contains(result, "└─") {
		t.Errorf("Expected tree characters in dict output, got: %v", result)
	}
	if !contains(result, "key1") || !contains(result, "100") {
		t.Errorf("Expected dict entries in output, got: %v", result)
	}
}

func TestRenderValue_NestedStructures(t *testing.T) {
	// Create a nested structure: {"items": [1, 2, 3]}
	innerList := starlark.NewList([]starlark.Value{
		starlark.MakeInt(1),
		starlark.MakeInt(2),
		starlark.MakeInt(3),
	})
	
	dict := starlark.NewDict(1)
	dict.SetKey(starlark.String("items"), innerList)

	result := RenderValue(dict)
	
	// Should contain nested tree structure
	if !contains(result, "items") {
		t.Errorf("Expected 'items' key in output, got: %v", result)
	}
	if !contains(result, "1") || !contains(result, "2") || !contains(result, "3") {
		t.Errorf("Expected nested list values in output, got: %v", result)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestRenderValue_FunctionWithDoc(t *testing.T) {
	// Create a simple function with documentation
	src := `
def greet(name, greeting="Hello"):
    """Greets a person"""
    return greeting + ", " + name
`
	thread := &starlark.Thread{Name: "test"}
	predeclared := make(starlark.StringDict)
	
	globals, err := starlark.ExecFile(thread, "test.star", src, predeclared)
	if err != nil {
		t.Fatalf("Failed to execute starlark: %v", err)
	}
	
	fn, ok := globals["greet"]
	if !ok {
		t.Fatalf("Function 'greet' not found in globals")
	}
	if fn == nil {
		t.Fatalf("Function 'greet' is nil")
	}
	result := RenderValue(fn)
	
	// Should contain function signature
	if !contains(result, "greet") {
		t.Errorf("Expected function name in output, got: %v", result)
	}
	if !contains(result, "name") {
		t.Errorf("Expected parameter name in output, got: %v", result)
	}
	if !contains(result, "greeting") {
		t.Errorf("Expected parameter with default in output, got: %v", result)
	}
	// Should contain doc string at top level
	if !contains(result, "Greets a person") {
		t.Errorf("Expected docstring in output, got: %v", result)
	}
}

func TestRenderValue_FunctionInNestedStructure(t *testing.T) {
	// Create a function
	src := `
def helper():
    """This is a helper"""
    pass
`
	thread := &starlark.Thread{Name: "test"}
	predeclared := make(starlark.StringDict)
	
	globals, err := starlark.ExecFile(thread, "test.star", src, predeclared)
	if err != nil {
		t.Fatalf("Failed to execute starlark: %v", err)
	}
	
	fn, ok := globals["helper"]
	if !ok {
		t.Fatalf("Function 'helper' not found in globals")
	}
	
	// Put function in a list (nested context)
	list := starlark.NewList([]starlark.Value{fn})
	result := RenderValue(list)
	
	// Should contain function signature
	if !contains(result, "helper") {
		t.Errorf("Expected function name in output, got: %v", result)
	}
	// Should NOT contain doc string when nested
	if contains(result, "This is a helper") {
		t.Errorf("Should not show docstring in nested context, got: %v", result)
	}
}
