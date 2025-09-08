package refs

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// TestValidNormalizedRefs confirms that a set of normalized refs can be parsed and printed
// without changing.
func TestValidNormalizedRefs(t *testing.T) {
	var refs = []string{
		"",
		"github.com/org/repo/-/path/to/package/@/deploy/prod#output/host",
		"github.com/org/repo/-/path/to/package/@v1/deploy/prod#output/host",
		"github.com/org/repo/-/path/to/package",
		"./-/path/to/package",
		"./-/path/to/package/@/deploy/prod#output/host",
		"./@/task/build#output/output1",

		"github.com/ocu-project/ocu/-/package.ocu.star/@ABC123/deploy/staging/ABC123/logs",
		"github.com/ocu-project/ocu/-/package.ocu.star/@ABC123/deploy/staging/ABC123/outputs",
		"github.com/org/repo/-/package.ocu.star/@v1/deploy/prod#output/host",

		// Global values, can be used for environments
		"@/environment/production",
		// Global custom value in a repository
		"@/environment/staging#attributes/type",
	}
	for _, ref := range refs {
		t.Run(ref, func(t *testing.T) {
			pr, err := Parse(ref)
			if err != nil {
				t.Errorf("Parse(%q) returned error: %v", ref, err)
				return
			}
			if ref != pr.String() {
				t.Errorf("expected %v, got %v", ref, pr.String())
				return
			}
		})
	}
}

func TestRelativeTo(t *testing.T) {
	var tests = []struct {
		ref        string
		relativeTo string
		expected   string
	}{
		{
			ref:        "./task/build#output/output1",
			relativeTo: "github.com/org/repo/-/path/to/package/@abc123",
			expected:   "github.com/org/repo/-/path/to/package/@abc123/task/build#output/output1",
		},
		{
			ref:        "./@/task/build#output/output1",
			relativeTo: "github.com/org/repo/-/path/to/package/@abc123",
			expected:   "github.com/org/repo/-/path/to/package/@/task/build#output/output1",
		},
		{
			ref:        "./-/path/to/package/@/task/build#output/output1",
			relativeTo: "github.com/org/repo/-/path/to/package/@abc123",
			expected:   "github.com/org/repo/-/path/to/package/@/task/build#output/output1",
		},
		{
			ref:        "github.com/org/repo2/-/path/to/package2/@/deploy/prod#output/host",
			relativeTo: "github.com/org/repo/-/path/to/package/@abc123",
			expected:   "github.com/org/repo2/-/path/to/package2/@/deploy/prod#output/host",
		},
		{
			ref:        "@/environment/production",
			relativeTo: "github.com/org/repo/-/path/to/package/@abc123",
			expected:   "@/environment/production",
		},
		{
			ref:        "./@v3",
			relativeTo: "minimal/repo.git/-/",
			expected:   "minimal/repo.git/-/@v3",
		},
	}

	for _, test := range tests {
		t.Run(test.ref, func(t *testing.T) {
			pr, err := Parse(test.ref)
			if err != nil {
				t.Errorf("Parse(%q) returned error: %v", test.ref, err)
				return
			}
			prel, err := Parse(test.relativeTo)
			if err != nil {
				t.Errorf("Parse(%q) returned error: %v", test.relativeTo, err)
				return
			}

			prr, err := pr.RelativeTo(prel)
			if err != nil {
				t.Errorf("RelativeTo(%q) returned error: %v", test.relativeTo, err)
				return
			}
			if prr.String() != test.expected {
				t.Log(prr.DebugString())
				t.Errorf("expected %v, got %v", test.expected, prr.String())
				return
			}
		})
	}
}

func TestRefStructure(t *testing.T) {
	var refs = []struct {
		ref      string
		expected Ref
	}{
		{
			ref:      "",
			expected: Ref{},
		},
		{
			ref: "github.com/org/repo/-/path/to/package/@/deploy/prod#output/host",
			expected: Ref{
				Repo:        "github.com/org/repo",
				Filename:    "path/to/package",
				SubPathType: "deploy",
				SubPath:     "prod",
				Fragment:    "output/host",
				Release:     CurrentRelease,
			},
		},
		{
			ref: "github.com/org/repo/-/path/to/package/@/deploy/prod/ABCDEF/logs",
			expected: Ref{
				Repo:        "github.com/org/repo",
				Filename:    "path/to/package",
				Release:     CurrentRelease,
				SubPathType: "deploy",
				SubPath:     "prod/ABCDEF/logs",
			},
		},
		{
			ref: "path/to/package",
			expected: Ref{
				Filename: "path/to/package",
			},
		},
		{
			ref: "github.com/org/repo/-/path/to/package",
			expected: Ref{
				Repo:     "github.com/org/repo",
				Filename: "path/to/package",
			},
		},
		{
			ref: "./-/path/to/package/@/deploy/prod#output/host",
			expected: Ref{
				Repo:        ".",
				Filename:    "path/to/package",
				SubPathType: "deploy",
				SubPath:     "prod",
				Fragment:    "output/host",
				Release:     CurrentRelease,
			},
		},
		{
			ref: "./-/path/to/package/@/deploy/prod#output/host",
			expected: Ref{
				Repo:        ".",
				Filename:    "path/to/package",
				SubPathType: "deploy",
				SubPath:     "prod",
				Fragment:    "output/host",
				Release:     CurrentRelease,
			},
		},
		{
			ref: "./-/path/to/package/@/task/build#output/output1",
			expected: Ref{
				Repo:        ".",
				Filename:    "path/to/package",
				SubPathType: "task",
				SubPath:     "build",
				Fragment:    "output/output1",
				Release:     CurrentRelease,
			},
		},
		{
			ref: "github.com/org/repo/-/path/to/package/@v1/deploy/prod#output/host",
			expected: Ref{
				Repo:        "github.com/org/repo",
				Filename:    "path/to/package",
				Release:     Release("v1"),
				SubPathType: "deploy",
				SubPath:     "prod",
				Fragment:    "output/host",
			},
		},
		{
			ref: "github.com/org/repo/-/path/to/package/@v1",
			expected: Ref{
				Repo:     "github.com/org/repo",
				Filename: "path/to/package",
				Release:  Release("v1"),
			},
		},
		{
			ref: "github.com/org/repo/-/@v1",
			expected: Ref{
				Repo:     "github.com/org/repo",
				Filename: "",
				Release:  Release("v1"),
			},
		},
		{
			ref: "github.com/org/repo/-/",
			expected: Ref{
				Repo:     "github.com/org/repo",
				Filename: "",
			},
		},
		{
			ref: "github.com/org/repo/-/@abc",
			expected: Ref{
				Repo:     "github.com/org/repo",
				Filename: "",
				Release:  Release("abc"),
			},
		},
		{
			ref: "frontend/@",
			expected: Ref{
				Filename: "frontend",
				Release:  CurrentRelease,
			},
		},
		{
			ref: "github.com/ocu-project/ocu/-/@/deploy/staging",
			expected: Ref{
				Repo:        "github.com/ocu-project/ocu",
				Filename:    "",
				SubPathType: "deploy",
				SubPath:     "staging",
				Release:     Release(""),
			},
		},
		{
			ref: "github.com/ocu-project/ocu/-/@/deploy/staging",
			expected: Ref{
				Repo:        "github.com/ocu-project/ocu",
				Filename:    "",
				SubPathType: "deploy",
				SubPath:     "staging",
				Release:     Release(""),
			},
		},
		{
			ref: "./@/task/build#output/output1",
			expected: Ref{
				Repo:        "",
				Filename:    ".",
				SubPathType: "task",
				SubPath:     "build",
				Fragment:    "output/output1",
				Release:     CurrentRelease,
			},
		},
		{
			ref: "@/environment/production",
			expected: Ref{
				Global:      true,
				Release:     CurrentRelease,
				SubPathType: SubPathTypeEnvironment,
				SubPath:     "production",
			},
		},
		{
			ref: "@/environment/production",
			expected: Ref{
				Global:      true,
				Release:     Release(""),
				SubPathType: SubPathTypeEnvironment,
				SubPath:     "production",
			},
		},
		{
			ref: "@/environment/staging#attributes/type",
			expected: Ref{
				Global:      true,
				Release:     CurrentRelease,
				SubPathType: SubPathTypeEnvironment,
				SubPath:     "staging",
				Fragment:    "attributes/type",
			},
		},
		{
			ref: "./-/path/to/package",
			expected: Ref{
				Repo:     ".",
				Filename: "path/to/package",
			},
		},
		{
			ref: "./task/build#output/output1",
			expected: Ref{
				Repo:        "",
				Filename:    ".",
				SubPathType: "task",
				SubPath:     "build",
				Fragment:    "output/output1",
				Release:     Release(""),
			},
		},
		{
			ref: "minimal/repo/-/package.ocu.star/@commitid.01JY1YT9R0EDV5EKQH5FVZF64B/custom/approval",
			expected: Ref{
				Repo:        "minimal/repo",
				Filename:    "package.ocu.star",
				Release:     Release("commitid.01JY1YT9R0EDV5EKQH5FVZF64B"),
				SubPathType: "custom",
				SubPath:     "approval",
			},
		},
		{
			ref: "//package.ocu.star/@r5/custom/approval",
			expected: Ref{
				Repo:        ".",
				Filename:    "package.ocu.star",
				Release:     Release("r5"),
				SubPathType: "custom",
				SubPath:     "approval",
			},
		},
	}
	for _, ref := range refs {
		t.Run(ref.ref, func(t *testing.T) {
			out, err := Parse(ref.ref)
			if err != nil {
				t.Errorf("Parse(%q) returned error: %v", ref.ref, err)
				return
			}
			if !cmp.Equal(out, ref.expected, cmpopts.IgnoreUnexported(Ref{})) {
				t.Errorf("Parse(%q): %v", ref.ref, cmp.Diff(out, ref.expected))
				return
			}

			if out.String() == "" {
				return
			}

			// Create a temporary directory that will be cleaned up after the test
			tempDir, err := os.MkdirTemp("", "pathtest-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir) // Clean up after test

			// Test path to use
			testPath := out.String()
			testContent := "test content\n"

			// Parse the URL
			parsedURL, err := url.Parse(testPath)
			if err != nil {
				t.Fatalf("Failed to parse path %q: %v", testPath, err)
			}

			dirPath := filepath.Join(tempDir, parsedURL.Host, parsedURL.Path)

			// Create all directories in the path
			err = os.MkdirAll(filepath.Dir(dirPath), 0755)
			if err != nil {
				t.Fatalf("Failed to create directories: %v", err)
			}

			// Write test file
			err = os.WriteFile(dirPath, []byte(testContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Read back the file
			content, err := os.ReadFile(dirPath)
			if err != nil {
				t.Fatalf("Failed to read test file: %v", err)
			}

			// Verify content
			if string(content) != testContent {
				t.Errorf("Content mismatch. Got %q, want %q", string(content), testContent)
			}
		})
	}
}

func TestInvalidRefs(t *testing.T) {
	var refs = []string{
		"repo.git/package/@/invalid/sub/path",
		"repo.git/package@/deploy/hello",
	}
	for _, ref := range refs {
		t.Run(ref, func(t *testing.T) {
			_, err := Parse(ref)
			if err == nil {
				t.Errorf("expected an error")
			}
		})
	}
}

func TestRefNormalization(t *testing.T) {
	// Normalization should remove trailing slashes
	var refsToNormalized = map[string]string{
		"./task/build/": "./task/build",
		"github.com/example/example/-/path/to/package/@/task/build#output/output1/": "github.com/example/example/-/path/to/package/@/task/build#output/output1",
		"github.com/example/example/-/path/to/package/@/task/build/":                "github.com/example/example/-/path/to/package/@/task/build",
		"github.com/example/example/-/path/to/package/@/call/build/":                "github.com/example/example/-/path/to/package/@/task/build",
	}
	for ref, expected := range refsToNormalized {
		t.Run(ref, func(t *testing.T) {
			pr, err := Parse(ref)
			if err != nil {
				t.Errorf("Parse(%q) returned error: %v", ref, err)
				return
			}
			if pr.String() != expected {
				t.Errorf("expected %v, got %v", expected, pr.String())
				return
			}
		})
	}
}

var CurrentRelease = Release("")
