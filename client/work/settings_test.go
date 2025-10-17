package work

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/ocuroot/ocuroot/sdk"
	"go.starlark.net/starlark"
)

func TestUnmarshalFromEnvVars(t *testing.T) {
	type EnvTestStruct struct {
		StringField  string  `env:"TEST_STRING"`
		BoolField    bool    `env:"TEST_BOOL"`
		IntField     int     `env:"TEST_INT"`
		Int64Field   int64   `env:"TEST_INT64"`
		UintField    uint    `env:"TEST_UINT"`
		Uint64Field  uint64  `env:"TEST_UINT64"`
		Float64Field float64 `env:"TEST_FLOAT64"`
		Float32Field float32 `env:"TEST_FLOAT32"`
		NoTagField   string
		NoEnvField   string `env:"TEST_MISSING"`
	}

	var tests = []struct {
		name     string
		in       []string
		ptr      any
		expected any
		wantErr  bool
	}{
		{
			name:     "empty_env",
			in:       []string{},
			ptr:      new(EnvTestStruct),
			expected: &EnvTestStruct{},
		},
		{
			name: "string_field",
			in:   []string{"TEST_STRING=hello world"},
			ptr:  new(EnvTestStruct),
			expected: &EnvTestStruct{
				StringField: "hello world",
			},
		},
		{
			name: "bool_true",
			in:   []string{"TEST_BOOL=true"},
			ptr:  new(EnvTestStruct),
			expected: &EnvTestStruct{
				BoolField: true,
			},
		},
		{
			name: "bool_false",
			in:   []string{"TEST_BOOL=false"},
			ptr:  new(EnvTestStruct),
			expected: &EnvTestStruct{
				BoolField: false,
			},
		},
		{
			name: "bool_1",
			in:   []string{"TEST_BOOL=1"},
			ptr:  new(EnvTestStruct),
			expected: &EnvTestStruct{
				BoolField: true,
			},
		},
		{
			name: "bool_0",
			in:   []string{"TEST_BOOL=0"},
			ptr:  new(EnvTestStruct),
			expected: &EnvTestStruct{
				BoolField: false,
			},
		},
		{
			name: "int_field",
			in:   []string{"TEST_INT=42"},
			ptr:  new(EnvTestStruct),
			expected: &EnvTestStruct{
				IntField: 42,
			},
		},
		{
			name: "int_negative",
			in:   []string{"TEST_INT=-100"},
			ptr:  new(EnvTestStruct),
			expected: &EnvTestStruct{
				IntField: -100,
			},
		},
		{
			name: "int64_field",
			in:   []string{"TEST_INT64=9223372036854775807"},
			ptr:  new(EnvTestStruct),
			expected: &EnvTestStruct{
				Int64Field: 9223372036854775807,
			},
		},
		{
			name: "uint_field",
			in:   []string{"TEST_UINT=100"},
			ptr:  new(EnvTestStruct),
			expected: &EnvTestStruct{
				UintField: 100,
			},
		},
		{
			name: "uint64_field",
			in:   []string{"TEST_UINT64=1000000000000"},
			ptr:  new(EnvTestStruct),
			expected: &EnvTestStruct{
				Uint64Field: 1000000000000,
			},
		},
		{
			name: "float64_field",
			in:   []string{"TEST_FLOAT64=3.14159"},
			ptr:  new(EnvTestStruct),
			expected: &EnvTestStruct{
				Float64Field: 3.14159,
			},
		},
		{
			name: "float32_field",
			in:   []string{"TEST_FLOAT32=2.71828"},
			ptr:  new(EnvTestStruct),
			expected: &EnvTestStruct{
				Float32Field: 2.71828,
			},
		},
		{
			name: "multiple_fields",
			in: []string{
				"TEST_STRING=test",
				"TEST_BOOL=true",
				"TEST_INT=42",
				"TEST_FLOAT64=3.14",
			},
			ptr: new(EnvTestStruct),
			expected: &EnvTestStruct{
				StringField:  "test",
				BoolField:    true,
				IntField:     42,
				Float64Field: 3.14,
			},
		},
		{
			name: "env_with_equals_in_value",
			in:   []string{"TEST_STRING=key=value"},
			ptr:  new(EnvTestStruct),
			expected: &EnvTestStruct{
				StringField: "key=value",
			},
		},
		{
			name: "env_without_equals",
			in:   []string{"TEST_STRING"},
			ptr:  new(EnvTestStruct),
			expected: &EnvTestStruct{},
		},
		{
			name: "missing_env_var",
			in:   []string{"OTHER_VAR=value"},
			ptr:  new(EnvTestStruct),
			expected: &EnvTestStruct{},
		},
		{
			name:    "invalid_bool",
			in:      []string{"TEST_BOOL=invalid"},
			ptr:     new(EnvTestStruct),
			wantErr: true,
		},
		{
			name:    "invalid_int",
			in:      []string{"TEST_INT=not_a_number"},
			ptr:     new(EnvTestStruct),
			wantErr: true,
		},
		{
			name:    "invalid_float",
			in:      []string{"TEST_FLOAT64=not_a_float"},
			ptr:     new(EnvTestStruct),
			wantErr: true,
		},
		{
			name:    "uint_negative",
			in:      []string{"TEST_UINT=-1"},
			ptr:     new(EnvTestStruct),
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := UnmarshalFromEnvVars(test.in, test.ptr)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !cmp.Equal(test.expected, test.ptr) {
				t.Errorf("output not as expected\n%s", cmp.Diff(test.expected, test.ptr))
			}
		})
	}
}

func TestUnmarshalFromEnvVarsUnsupportedTypes(t *testing.T) {
	type UnsupportedStruct struct {
		SliceField []string          `env:"TEST_SLICE"`
		MapField   map[string]string `env:"TEST_MAP"`
	}

	var tests = []struct {
		name string
		in   []string
		ptr  any
	}{
		{
			name: "slice_not_supported",
			in:   []string{"TEST_SLICE=value"},
			ptr:  new(UnsupportedStruct),
		},
		{
			name: "map_not_supported",
			in:   []string{"TEST_MAP=value"},
			ptr:  &UnsupportedStruct{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := UnmarshalFromEnvVars(test.in, test.ptr)
			if err == nil {
				t.Fatal("expected error for unsupported type but got none")
			}
			if !strings.Contains(err.Error(), "unsupported type") {
				t.Errorf("expected 'unsupported type' error, got: %v", err)
			}
		})
	}
}

func TestUnmarshalFromEnvVarsSettings(t *testing.T) {
	var tests = []struct {
		name     string
		in       []string
		expected Settings
	}{
		{
			name: "repo_alias",
			in:   []string{"OCU_CFG_repo_alias=my-repo"},
			expected: Settings{
				RepoAlias: "my-repo",
			},
		},
		{
			name: "no_matching_env_vars",
			in:   []string{"OTHER_VAR=value"},
			expected: Settings{},
		},
		{
			name: "empty_env",
			in:   []string{},
			expected: Settings{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var s Settings
			err := UnmarshalFromEnvVars(test.in, &s)
			if err != nil {
				t.Fatal(err)
			}
			if !cmp.Equal(test.expected, s) {
				t.Errorf("output not as expected\n%s", cmp.Diff(test.expected, s))
			}
		})
	}
}

func TestUnmarshalFromValueNone(t *testing.T) {
	var tests = []struct {
		name     string
		in       starlark.Value
		ptr      any
		expected any
	}{
		{
			name:     "none_to_string",
			in:       starlark.None,
			ptr:      new(string),
			expected: new(string), // empty string
		},
		{
			name:     "none_to_int",
			in:       starlark.None,
			ptr:      new(int),
			expected: new(int), // zero
		},
		{
			name:     "none_to_bool",
			in:       starlark.None,
			ptr:      new(bool),
			expected: new(bool), // false
		},
		{
			name:     "none_to_map",
			in:       starlark.None,
			ptr:      new(map[string]string),
			expected: new(map[string]string), // nil map
		},
		{
			name:     "none_to_slice",
			in:       starlark.None,
			ptr:      new([]string),
			expected: new([]string), // nil slice
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := UnmarshalFromValue(test.in, test.ptr)
			if err != nil {
				t.Fatal(err)
			}
			if !cmp.Equal(test.expected, test.ptr) {
				t.Errorf("output not as expected\n%s", cmp.Diff(test.expected, test.ptr))
			}
		})
	}
}

func TestUnmarshalFromValueStorageBackend(t *testing.T) {
	var tests = []struct {
		name     string
		in       starlark.Value
		ptr      any
		expected any
	}{
		{
			name: "empty_storage_backend",
			in:   starlark.NewDict(0),
			ptr:  new(sdk.StorageBackend),
			expected: &sdk.StorageBackend{},
		},
		{
			name: "git_storage_backend",
			in: func() starlark.Value {
				gitDict := starlark.NewDict(2)
				gitDict.SetKey(starlark.String("remote_url"), starlark.String("https://github.com/example/repo.git"))
				gitDict.SetKey(starlark.String("branch"), starlark.String("main"))
				
				stateDict := starlark.NewDict(1)
				stateDict.SetKey(starlark.String("git"), gitDict)
				
				return stateDict
			}(),
			ptr: new(sdk.StorageBackend),
			expected: func() *sdk.StorageBackend {
				sb := &sdk.StorageBackend{}
				sb.Git = &struct {
					RemoteURL    string            `json:"remote_url" starlark:"remote_url"`
					Branch       string            `json:"branch" starlark:"branch"`
					SupportFiles map[string]string `json:"support_files,omitempty" starlark:"support_files,omitempty"`
					PathPrefix   string            `json:"path_prefix,omitempty" starlark:"path_prefix,omitempty"`
					CreateBranch bool              `json:"create_branch,omitempty" starlark:"create_branch,omitempty"`
				}{
					RemoteURL: "https://github.com/example/repo.git",
					Branch:    "main",
				}
				return sb
			}(),
		},
		{
			name: "fs_storage_backend",
			in: func() starlark.Value {
				fsDict := starlark.NewDict(1)
				fsDict.SetKey(starlark.String("path"), starlark.String("/tmp/store"))
				
				stateDict := starlark.NewDict(1)
				stateDict.SetKey(starlark.String("fs"), fsDict)
				
				return stateDict
			}(),
			ptr: new(sdk.StorageBackend),
			expected: func() *sdk.StorageBackend {
				sb := &sdk.StorageBackend{}
				sb.Fs = &struct {
					Path string `json:"path" starlark:"path"`
				}{
					Path: "/tmp/store",
				}
				return sb
			}(),
		},
		{
			name: "git_with_create_branch",
			in: func() starlark.Value {
				gitDict := starlark.NewDict(3)
				gitDict.SetKey(starlark.String("remote_url"), starlark.String("https://github.com/example/repo.git"))
				gitDict.SetKey(starlark.String("branch"), starlark.String("state"))
				gitDict.SetKey(starlark.String("create_branch"), starlark.Bool(true))
				
				stateDict := starlark.NewDict(1)
				stateDict.SetKey(starlark.String("git"), gitDict)
				
				return stateDict
			}(),
			ptr: new(sdk.StorageBackend),
			expected: func() *sdk.StorageBackend {
				sb := &sdk.StorageBackend{}
				sb.Git = &struct {
					RemoteURL    string            `json:"remote_url" starlark:"remote_url"`
					Branch       string            `json:"branch" starlark:"branch"`
					SupportFiles map[string]string `json:"support_files,omitempty" starlark:"support_files,omitempty"`
					PathPrefix   string            `json:"path_prefix,omitempty" starlark:"path_prefix,omitempty"`
					CreateBranch bool              `json:"create_branch,omitempty" starlark:"create_branch,omitempty"`
				}{
					RemoteURL:    "https://github.com/example/repo.git",
					Branch:       "state",
					CreateBranch: true,
				}
				return sb
			}(),
		},
		{
			name: "git_with_none_support_files",
			in: func() starlark.Value {
				gitDict := starlark.NewDict(3)
				gitDict.SetKey(starlark.String("remote_url"), starlark.String("https://github.com/example/repo.git"))
				gitDict.SetKey(starlark.String("branch"), starlark.String("state"))
				gitDict.SetKey(starlark.String("support_files"), starlark.None)
				
				stateDict := starlark.NewDict(1)
				stateDict.SetKey(starlark.String("git"), gitDict)
				
				return stateDict
			}(),
			ptr: new(sdk.StorageBackend),
			expected: func() *sdk.StorageBackend {
				sb := &sdk.StorageBackend{}
				sb.Git = &struct {
					RemoteURL    string            `json:"remote_url" starlark:"remote_url"`
					Branch       string            `json:"branch" starlark:"branch"`
					SupportFiles map[string]string `json:"support_files,omitempty" starlark:"support_files,omitempty"`
					PathPrefix   string            `json:"path_prefix,omitempty" starlark:"path_prefix,omitempty"`
					CreateBranch bool              `json:"create_branch,omitempty" starlark:"create_branch,omitempty"`
				}{
					RemoteURL: "https://github.com/example/repo.git",
					Branch:    "state",
					// SupportFiles should be nil (zero value for map)
				}
				return sb
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := UnmarshalFromValue(test.in, test.ptr)
			if err != nil {
				t.Fatal(err)
			}
			if !cmp.Equal(test.expected, test.ptr) {
				t.Errorf("output not as expected\n%s", cmp.Diff(test.expected, test.ptr))
			}
		})
	}
}

func TestUnmarshalStringDictWithStorageBackends(t *testing.T) {
	var tests = []struct {
		name     string
		in       starlark.StringDict
		expected Settings
	}{
		{
			name: "settings_with_state_store",
			in: starlark.StringDict{
				"repo_alias": starlark.String("my-repo"),
				"state_store": func() starlark.Value {
					gitDict := starlark.NewDict(2)
					gitDict.SetKey(starlark.String("remote_url"), starlark.String("https://github.com/example/repo.git"))
					gitDict.SetKey(starlark.String("branch"), starlark.String("state"))
					
					stateDict := starlark.NewDict(1)
					stateDict.SetKey(starlark.String("git"), gitDict)
					
					return stateDict
				}(),
			},
			expected: func() Settings {
				sb := &sdk.StorageBackend{}
				sb.Git = &struct {
					RemoteURL    string            `json:"remote_url" starlark:"remote_url"`
					Branch       string            `json:"branch" starlark:"branch"`
					SupportFiles map[string]string `json:"support_files,omitempty" starlark:"support_files,omitempty"`
					PathPrefix   string            `json:"path_prefix,omitempty" starlark:"path_prefix,omitempty"`
					CreateBranch bool              `json:"create_branch,omitempty" starlark:"create_branch,omitempty"`
				}{
					RemoteURL: "https://github.com/example/repo.git",
					Branch:    "state",
				}
				return Settings{
					RepoAlias: "my-repo",
					State:     sb,
				}
			}(),
		},
		{
			name: "settings_with_state_and_intent_stores",
			in: starlark.StringDict{
				"repo_alias": starlark.String("my-repo"),
				"state_store": func() starlark.Value {
					gitDict := starlark.NewDict(2)
					gitDict.SetKey(starlark.String("remote_url"), starlark.String("https://github.com/example/repo.git"))
					gitDict.SetKey(starlark.String("branch"), starlark.String("state"))
					
					stateDict := starlark.NewDict(1)
					stateDict.SetKey(starlark.String("git"), gitDict)
					
					return stateDict
				}(),
				"intent_store": func() starlark.Value {
					gitDict := starlark.NewDict(2)
					gitDict.SetKey(starlark.String("remote_url"), starlark.String("https://github.com/example/repo.git"))
					gitDict.SetKey(starlark.String("branch"), starlark.String("intent"))
					
					intentDict := starlark.NewDict(1)
					intentDict.SetKey(starlark.String("git"), gitDict)
					
					return intentDict
				}(),
			},
			expected: func() Settings {
				stateSB := &sdk.StorageBackend{}
				stateSB.Git = &struct {
					RemoteURL    string            `json:"remote_url" starlark:"remote_url"`
					Branch       string            `json:"branch" starlark:"branch"`
					SupportFiles map[string]string `json:"support_files,omitempty" starlark:"support_files,omitempty"`
					PathPrefix   string            `json:"path_prefix,omitempty" starlark:"path_prefix,omitempty"`
					CreateBranch bool              `json:"create_branch,omitempty" starlark:"create_branch,omitempty"`
				}{
					RemoteURL: "https://github.com/example/repo.git",
					Branch:    "state",
				}
				intentSB := &sdk.StorageBackend{}
				intentSB.Git = &struct {
					RemoteURL    string            `json:"remote_url" starlark:"remote_url"`
					Branch       string            `json:"branch" starlark:"branch"`
					SupportFiles map[string]string `json:"support_files,omitempty" starlark:"support_files,omitempty"`
					PathPrefix   string            `json:"path_prefix,omitempty" starlark:"path_prefix,omitempty"`
					CreateBranch bool              `json:"create_branch,omitempty" starlark:"create_branch,omitempty"`
				}{
					RemoteURL: "https://github.com/example/repo.git",
					Branch:    "intent",
				}
				return Settings{
					RepoAlias: "my-repo",
					State:     stateSB,
					Intent:    intentSB,
				}
			}(),
		},
		{
			name: "settings_without_storage_backends",
			in: starlark.StringDict{
				"repo_alias": starlark.String("my-repo"),
			},
			expected: Settings{
				RepoAlias: "my-repo",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var s Settings
			err := UnmarshalFromStringDict(test.in, &s)
			if err != nil {
				t.Fatal(err)
			}
			if !cmp.Equal(test.expected, s) {
				t.Errorf("output not as expected\n%s", cmp.Diff(test.expected, s))
			}
		})
	}
}

func TestUnmarshalStringDict(t *testing.T) {
	var tests = []struct {
		name     string
		in       starlark.StringDict
		ptr      any
		expected any
	}{
		{
			name:     "empty",
			in:       starlark.StringDict{},
			ptr:      new(Settings),
			expected: &Settings{},
		},
		{
			name: "single_field",
			in: starlark.StringDict{
				"repo_alias": starlark.String("repo_alias"),
			},
			ptr: new(Settings),
			expected: &Settings{
				RepoAlias: "repo_alias",
			},
		},
		{
			name: "unknown_field_ignored",
			in: starlark.StringDict{
				"repo_alias":   starlark.String("my-repo"),
				"unknown_field": starlark.String("ignored"),
			},
			ptr: new(Settings),
			expected: &Settings{
				RepoAlias: "my-repo",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := UnmarshalFromStringDict(test.in, test.ptr)
			if err != nil {
				t.Fatal(err)
			}
			if !cmp.Equal(test.expected, test.ptr) {
				t.Errorf("output not as expected\n%s", cmp.Diff(test.expected, test.ptr))
			}
		})
	}
}

type TestStruct struct {
	StringField  string  `starlark:"string_field"`
	BoolField    bool    `starlark:"bool_field"`
	IntField     int     `starlark:"int_field"`
	Int64Field   int64   `starlark:"int64_field"`
	UintField    uint    `starlark:"uint_field"`
	Float64Field float64 `starlark:"float64_field"`
	Float32Field float32 `starlark:"float32_field"`
}

type ComplexTestStruct struct {
	Name       string            `starlark:"name"`
	Tags       []string          `starlark:"tags"`
	Counts     []int             `starlark:"counts"`
	Metadata   map[string]string `starlark:"metadata"`
	Settings   map[string]int    `starlark:"settings"`
	NestedList [][]string        `starlark:"nested_list"`
}

func TestUnmarshalStringDictMultipleTypes(t *testing.T) {
	var tests = []struct {
		name     string
		in       starlark.StringDict
		ptr      any
		expected any
	}{
		{
			name: "all_types",
			in: starlark.StringDict{
				"string_field":  starlark.String("test"),
				"bool_field":    starlark.Bool(true),
				"int_field":     starlark.MakeInt(42),
				"int64_field":   starlark.MakeInt(9223372036854775807),
				"uint_field":    starlark.MakeInt(100),
				"float64_field": starlark.Float(3.14),
				"float32_field": starlark.Float(2.71),
			},
			ptr: new(TestStruct),
			expected: &TestStruct{
				StringField:  "test",
				BoolField:    true,
				IntField:     42,
				Int64Field:   9223372036854775807,
				UintField:    100,
				Float64Field: 3.14,
				Float32Field: 2.71,
			},
		},
		{
			name: "partial_fields",
			in: starlark.StringDict{
				"string_field": starlark.String("partial"),
				"int_field":    starlark.MakeInt(99),
			},
			ptr: new(TestStruct),
			expected: &TestStruct{
				StringField: "partial",
				IntField:    99,
			},
		},
		{
			name: "complex_with_lists_and_maps",
			in: starlark.StringDict{
				"name": starlark.String("test-config"),
				"tags": starlark.NewList([]starlark.Value{
					starlark.String("tag1"),
					starlark.String("tag2"),
					starlark.String("tag3"),
				}),
				"counts": starlark.NewList([]starlark.Value{
					starlark.MakeInt(10),
					starlark.MakeInt(20),
					starlark.MakeInt(30),
				}),
				"metadata": func() starlark.Value {
					d := starlark.NewDict(2)
					d.SetKey(starlark.String("author"), starlark.String("alice"))
					d.SetKey(starlark.String("version"), starlark.String("1.0"))
					return d
				}(),
				"settings": func() starlark.Value {
					d := starlark.NewDict(2)
					d.SetKey(starlark.String("timeout"), starlark.MakeInt(30))
					d.SetKey(starlark.String("retries"), starlark.MakeInt(3))
					return d
				}(),
				"nested_list": starlark.NewList([]starlark.Value{
					starlark.NewList([]starlark.Value{
						starlark.String("a1"),
						starlark.String("a2"),
					}),
					starlark.NewList([]starlark.Value{
						starlark.String("b1"),
						starlark.String("b2"),
					}),
				}),
			},
			ptr: new(ComplexTestStruct),
			expected: &ComplexTestStruct{
				Name:       "test-config",
				Tags:       []string{"tag1", "tag2", "tag3"},
				Counts:     []int{10, 20, 30},
				Metadata:   map[string]string{"author": "alice", "version": "1.0"},
				Settings:   map[string]int{"timeout": 30, "retries": 3},
				NestedList: [][]string{{"a1", "a2"}, {"b1", "b2"}},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := UnmarshalFromStringDict(test.in, test.ptr)
			if err != nil {
				t.Fatal(err)
			}
			if !cmp.Equal(test.expected, test.ptr) {
				t.Errorf("output not as expected\n%s", cmp.Diff(test.expected, test.ptr))
			}
		})
	}
}

func TestUnmarshalFromValue(t *testing.T) {
	var tests = []struct {
		name     string
		in       starlark.Value
		ptr      any
		expected any
	}{
		{
			name:     "string",
			in:       starlark.String("hello"),
			ptr:      new(string),
			expected: func() *string { s := "hello"; return &s }(),
		},
		{
			name:     "bool_true",
			in:       starlark.Bool(true),
			ptr:      new(bool),
			expected: func() *bool { b := true; return &b }(),
		},
		{
			name:     "bool_false",
			in:       starlark.Bool(false),
			ptr:      new(bool),
			expected: func() *bool { b := false; return &b }(),
		},
		{
			name:     "int",
			in:       starlark.MakeInt(42),
			ptr:      new(int),
			expected: func() *int { i := 42; return &i }(),
		},
		{
			name:     "int64",
			in:       starlark.MakeInt(9223372036854775807),
			ptr:      new(int64),
			expected: func() *int64 { i := int64(9223372036854775807); return &i }(),
		},
		{
			name:     "uint",
			in:       starlark.MakeInt(100),
			ptr:      new(uint),
			expected: func() *uint { i := uint(100); return &i }(),
		},
		{
			name:     "uint64",
			in:       starlark.MakeInt(1000000000000),
			ptr:      new(uint64),
			expected: func() *uint64 { i := uint64(1000000000000); return &i }(),
		},
		{
			name:     "int_to_float64",
			in:       starlark.MakeInt(42),
			ptr:      new(float64),
			expected: func() *float64 { f := float64(42); return &f }(),
		},
		{
			name:     "int_to_float32",
			in:       starlark.MakeInt(42),
			ptr:      new(float32),
			expected: func() *float32 { f := float32(42); return &f }(),
		},
		{
			name:     "float64",
			in:       starlark.Float(3.14159),
			ptr:      new(float64),
			expected: func() *float64 { f := 3.14159; return &f }(),
		},
		{
			name:     "float32",
			in:       starlark.Float(2.71828),
			ptr:      new(float32),
			expected: func() *float32 { f := float32(2.71828); return &f }(),
		},
		{
			name: "list_of_strings",
			in: func() starlark.Value {
				return starlark.NewList([]starlark.Value{
					starlark.String("a"),
					starlark.String("b"),
					starlark.String("c"),
				})
			}(),
			ptr:      new([]string),
			expected: &[]string{"a", "b", "c"},
		},
		{
			name: "list_of_ints",
			in: func() starlark.Value {
				return starlark.NewList([]starlark.Value{
					starlark.MakeInt(1),
					starlark.MakeInt(2),
					starlark.MakeInt(3),
				})
			}(),
			ptr:      new([]int),
			expected: &[]int{1, 2, 3},
		},
		{
			name: "empty_list",
			in: func() starlark.Value {
				return starlark.NewList([]starlark.Value{})
			}(),
			ptr:      new([]string),
			expected: &[]string{},
		},
		{
			name: "list_of_bools",
			in: func() starlark.Value {
				return starlark.NewList([]starlark.Value{
					starlark.Bool(true),
					starlark.Bool(false),
					starlark.Bool(true),
				})
			}(),
			ptr:      new([]bool),
			expected: &[]bool{true, false, true},
		},
		{
			name: "dict_string_to_string",
			in: func() starlark.Value {
				d := starlark.NewDict(2)
				d.SetKey(starlark.String("key1"), starlark.String("value1"))
				d.SetKey(starlark.String("key2"), starlark.String("value2"))
				return d
			}(),
			ptr:      new(map[string]string),
			expected: &map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name: "dict_string_to_int",
			in: func() starlark.Value {
				d := starlark.NewDict(2)
				d.SetKey(starlark.String("one"), starlark.MakeInt(1))
				d.SetKey(starlark.String("two"), starlark.MakeInt(2))
				return d
			}(),
			ptr:      new(map[string]int),
			expected: &map[string]int{"one": 1, "two": 2},
		},
		{
			name: "dict_string_to_bool",
			in: func() starlark.Value {
				d := starlark.NewDict(2)
				d.SetKey(starlark.String("enabled"), starlark.Bool(true))
				d.SetKey(starlark.String("disabled"), starlark.Bool(false))
				return d
			}(),
			ptr:      new(map[string]bool),
			expected: &map[string]bool{"enabled": true, "disabled": false},
		},
		{
			name: "empty_dict",
			in: func() starlark.Value {
				return starlark.NewDict(0)
			}(),
			ptr:      new(map[string]string),
			expected: &map[string]string{},
		},
		{
			name: "dict_string_to_list",
			in: func() starlark.Value {
				d := starlark.NewDict(2)
				d.SetKey(starlark.String("list1"), starlark.NewList([]starlark.Value{
					starlark.String("a"),
					starlark.String("b"),
				}))
				d.SetKey(starlark.String("list2"), starlark.NewList([]starlark.Value{
					starlark.String("c"),
					starlark.String("d"),
				}))
				return d
			}(),
			ptr:      new(map[string][]string),
			expected: &map[string][]string{"list1": {"a", "b"}, "list2": {"c", "d"}},
		},
		{
			name: "list_of_dicts",
			in: func() starlark.Value {
				d1 := starlark.NewDict(1)
				d1.SetKey(starlark.String("name"), starlark.String("alice"))
				d2 := starlark.NewDict(1)
				d2.SetKey(starlark.String("name"), starlark.String("bob"))
				return starlark.NewList([]starlark.Value{d1, d2})
			}(),
			ptr:      new([]map[string]string),
			expected: &[]map[string]string{{"name": "alice"}, {"name": "bob"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := UnmarshalFromValue(test.in, test.ptr)
			if err != nil {
				t.Fatal(err)
			}
			if !cmp.Equal(test.expected, test.ptr) {
				t.Errorf("output not as expected\n%s", cmp.Diff(test.expected, test.ptr))
			}
		})
	}
}
