package refs

import (
	"testing"

	libglob "github.com/gobwas/glob"
)

func TestReduce(t *testing.T) {
	var tests = []struct {
		ref    string
		glob   string
		result string
	}{
		{
			ref:    "github.com/example/myrepo/-/path/to/release.ocu.star/@commitid/task/build",
			glob:   "**/@*",
			result: "github.com/example/myrepo/-/path/to/release.ocu.star/@commitid",
		},
		{
			ref:    "github.com/example/myrepo/-/path/to/release.ocu.star/@commitid",
			glob:   "**/@*",
			result: "github.com/example/myrepo/-/path/to/release.ocu.star/@commitid",
		},
		{
			ref:    "github.com/example/myrepo/-/path/to/release.ocu.star/@commitid/task/build",
			glob:   "**/@",
			result: "github.com/example/myrepo/-/path/to/release.ocu.star/@",
		},
		{
			ref:    "github.com/example/myrepo/-/path/to/release.ocu.star/@commitid/task/build",
			glob:   "**/@/task/*",
			result: "github.com/example/myrepo/-/path/to/release.ocu.star/@/task/build",
		},
	}

	for _, test := range tests {
		glob, err := libglob.Compile(test.glob, '/')
		if err != nil {
			t.Errorf("Compile(%q) error: %v", test.glob, err)
		}
		result, err := Reduce(test.ref, glob)
		if err != nil {
			t.Errorf("Reduce(%q, %q) error: %v", test.ref, test.glob, err)
		}
		if result != test.result {
			t.Errorf("Reduce(%q, %q) = %q, want %q", test.ref, test.glob, result, test.result)
		}
	}

}

func TestReduceNoMatch(t *testing.T) {
	tests := []struct {
		ref   string
		glob  string
		error string
	}{
		{
			ref:   "github.com/example/myrepo/-/path/to/release.ocu.star/@commitid/task/build",
			glob:  "**/@/deploy/*",
			error: "no match",
		},
		{
			ref:   "github.com/example/myrepo/-/path/to/release.ocu.star/@commitid",
			glob:  "**/@/task/*",
			error: "no match",
		},
	}

	for _, test := range tests {
		glob, err := libglob.Compile(test.glob, '/')
		if err != nil {
			t.Errorf("Compile(%q) error: %v", test.glob, err)
		}
		_, err = Reduce(test.ref, glob)
		if err == nil || err.Error() != test.error {
			t.Errorf("Reduce(%q, %q) = %v, want %v", test.ref, test.glob, err, test.error)
		}
	}

}
