package refstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/google/go-cmp/cmp"
)

func doTestBackendSetGet(t *testing.T, getBackend func() DocumentBackend) {
	backend := getBackend()
	results, err := backend.Get(context.Background(), []string{"test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 results, got %d", len(results))
	}
	if results[0].Doc != nil {
		t.Fatalf("expected nil doc, got '%s'", string(results[0].Doc.Body))
	}

	docPath := "test/@object.json"

	if err := backend.Set(context.Background(), "test", []SetRequest{
		{
			Path: docPath,
			Doc: &StorageObject{
				Kind: "test",
				Body: []byte(`"test"`),
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	results, err = backend.Get(context.Background(), []string{docPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Path != docPath {
		t.Fatalf("expected ref 'test', got '%s'", results[0].Path)
	}
	if results[0].Doc == nil || results[0].Doc.Body == nil {
		t.Fatalf("doc or body was nil: %+v", results[0])
	}
	if string(results[0].Doc.Body) != `"test"` {
		t.Fatalf("expected body 'test', got '%s'", string(results[0].Doc.Body))
	}

	// Delete a doc by setting it to nil
	err = backend.Set(context.Background(), "test", []SetRequest{
		{
			Path: docPath,
			Doc:  nil,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	results, err = backend.Get(context.Background(), []string{docPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Path != docPath {
		t.Fatalf("expected ref 'test', got '%s'", results[0].Path)
	}
	if results[0].Doc != nil {
		t.Fatalf("expected nil doc, got '%s'", string(results[0].Doc.Body))
	}

	// Send multiple set requests including a no-op deletion
	err = backend.Set(context.Background(), "multiple including no-op", []SetRequest{
		{
			Path: "missing/file.txt",
			Doc:  nil,
		},
		{
			Path: "new/file.txt",
			Doc: &StorageObject{
				Kind: "test",
				Body: []byte(`"test"`),
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to send no-op set: %v", err)
	}

	results, err = backend.Get(context.Background(), []string{"new/file.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Path != "new/file.txt" {
		t.Fatalf("expected ref 'test', got '%s'", results[0].Path)
	}
	if results[0].Doc == nil || results[0].Doc.Body == nil {
		t.Fatalf("doc or body was nil: %+v", results[0])
	}
	if string(results[0].Doc.Body) != `"test"` {
		t.Fatalf("expected body 'test', got '%s'", string(results[0].Doc.Body))
	}
}

func doTestBackendInfo(t *testing.T, getBackend func() DocumentBackend) {
	backend := getBackend()

	// Test GetBytes on non-existent file
	data, err := backend.GetBytes(context.Background(), storeInfoFile)
	if err != nil {
		t.Fatal(err)
	}
	if data != nil {
		t.Errorf("expected nil data, got %+v", data)
	}

	// Test SetBytes
	testInfo := &StoreInfo{Version: 2, Tags: map[string]struct{}{"test": {}}}
	infoBytes, err := json.Marshal(testInfo)
	if err != nil {
		t.Fatal(err)
	}

	if err := backend.SetBytes(context.Background(), storeInfoFile, infoBytes); err != nil {
		t.Fatal(err)
	}

	// Test GetBytes retrieves what was set
	data, err = backend.GetBytes(context.Background(), storeInfoFile)
	if err != nil {
		t.Fatal(err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}

	var retrievedInfo StoreInfo
	if err := json.Unmarshal(data, &retrievedInfo); err != nil {
		t.Fatal(err)
	}
	if retrievedInfo.Version != 2 {
		t.Errorf("expected version 2, got %d", retrievedInfo.Version)
	}
	if _, ok := retrievedInfo.Tags["test"]; !ok {
		t.Errorf("expected 'test' tag to be present")
	}

	// Perform a no-op change
	if err := backend.SetBytes(context.Background(), storeInfoFile, infoBytes); err != nil {
		t.Fatalf("no-op change failed %v", err)
	}

	// Test GetBytes retrieves what was set
	data, err = backend.GetBytes(context.Background(), storeInfoFile)
	if err != nil {
		t.Fatal(err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}

	// Ensure a real change is not impacted by the no-op
	testInfo = &StoreInfo{Version: 2, Tags: map[string]struct{}{"test": {}, "test2": {}}}
	infoBytes, err = json.Marshal(testInfo)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.SetBytes(context.Background(), storeInfoFile, infoBytes); err != nil {
		t.Fatalf("follow up change failed: %v", err)
	}

	// Test GetBytes retrieves what was set
	data, err = backend.GetBytes(context.Background(), storeInfoFile)
	if err != nil {
		t.Fatal(err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}
}

func expectNoError(t *testing.T, err error) {
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func doTestBackendMatch(t *testing.T, getBackend func() DocumentBackend) {
	var tests = []struct {
		name     string
		setup    func(DocumentBackend)
		reqs     []MatchRequest
		expect   []string
		checkErr func(*testing.T, error)
	}{
		{
			name: "match empty",
			setup: func(backend DocumentBackend) {
				backend.Set(context.Background(), "test", []SetRequest{})
			},
			reqs:     []MatchRequest{{Prefix: "subpath", Glob: "**"}},
			expect:   nil,
			checkErr: expectNoError,
		},
		{
			name: "match one of two",
			setup: func(backend DocumentBackend) {
				err := backend.Set(context.Background(), "test", []SetRequest{
					{
						Path: "subpath/one/@object.json",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
					{
						Path: "should/not/match",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
				})
				if err != nil {
					t.Fatal(err)
				}
			},
			reqs:     []MatchRequest{{Glob: "subpath/**"}},
			expect:   []string{"subpath/one/@object.json"},
			checkErr: expectNoError,
		},
		{
			name: "match prefix",
			setup: func(backend DocumentBackend) {
				err := backend.Set(context.Background(), "test", []SetRequest{
					{
						Path: "subpath/one/@object.json",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
					{
						Path: "should/not/match",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
				})
				if err != nil {
					t.Fatal(err)
				}
			},
			reqs:     []MatchRequest{{Prefix: "subpath", Glob: "**"}},
			expect:   []string{"subpath/one/@object.json"},
			checkErr: expectNoError,
		},
		{
			name: "match suffix",
			setup: func(backend DocumentBackend) {
				err := backend.Set(context.Background(), "test", []SetRequest{
					{
						Path: "subpath/one/@object.json",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
					{
						Path: "should/not/match",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
				})
				if err != nil {
					t.Fatal(err)
				}
			},
			reqs:     []MatchRequest{{Suffixes: []string{"/@object.json"}, Glob: "**"}},
			expect:   []string{"subpath/one/@object.json"},
			checkErr: expectNoError,
		},
		{
			name: "match prefix and suffix",
			setup: func(backend DocumentBackend) {
				err := backend.Set(context.Background(), "test", []SetRequest{
					{
						Path: "subpath/one/@object.json",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
					{
						Path: "should/not/match",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
				})
				if err != nil {
					t.Fatal(err)
				}
			},
			reqs:     []MatchRequest{{Prefix: "subpath", Suffixes: []string{"/@object.json"}, Glob: "**"}},
			expect:   []string{"subpath/one/@object.json"},
			checkErr: expectNoError,
		},
		{
			name: "match gitstate",
			setup: func(backend DocumentBackend) {
				err := backend.Set(context.Background(), "test", []SetRequest{
					{
						Path: "refs/@/environment/production/@object.json",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
					{
						Path: "refs/@/environment/production2/@object.json",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
					{
						Path: "refs/@/environment/staging/@object.json",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
					{
						Path: "refs/gitstate/repo/-/basic.ocu.star/@r1/@object.json",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
					{
						Path: "refs/gitstate/repo/-/basic.ocu.star/@r1/commit/commitid/@object.json",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
					{
						Path: "refs/gitstate/repo/-/basic.ocu.star/@r1/task/build/1/@object.json",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
					{
						Path: "refs/gitstate/repo/-/basic.ocu.star/@r1/task/build/1/status/pending/@object.json",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
					{
						Path: "refs/gitstate/repo/-/repo.ocu.star/@/@object.json",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
					{
						Path: "refs/gitstate/repo/-/repo.ocu.star/@commitid/@object.json",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
					{
						Path: "support.txt",
						Doc: &StorageObject{
							Kind: "test",
							Body: []byte(`"test"`),
						},
					},
				})
				if err != nil {
					t.Fatal(err)
				}
			},
			reqs:     []MatchRequest{{Prefix: "refs/", Suffixes: []string{"/@object.json"}, Glob: "gitstate/repo/-/basic.ocu.star/@r1/**/status/*"}},
			expect:   []string{"refs/gitstate/repo/-/basic.ocu.star/@r1/task/build/1/status/pending/@object.json"},
			checkErr: expectNoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log.SetOutput(logWriter(t))

			backend := getBackend()
			test.setup(backend)
			results, err := backend.Match(context.Background(), test.reqs)
			test.checkErr(t, err)
			if !cmp.Equal(results, test.expect) {
				t.Errorf("results do not match expected\n%v", cmp.Diff(results, test.expect))
			}
		})
	}
}

func doTestMultipleClients(t *testing.T, getBackend func() DocumentBackend) {
	backend1 := getBackend()
	backend2 := getBackend()

	ctx := t.Context()

	changes := make(chan SetRequest, 100)

	var wg sync.WaitGroup

	// Make changes in the background
	wg.Go(func() {
		count := 0
		for change := range changes {
			t.Logf("backend1, change %v", count)
			err := backend1.Set(ctx, "background update", []SetRequest{change})
			if err != nil {
				t.Errorf("backend1, update %v: %v", count, err)
			}
			count++
		}
	})

	wg.Go(func() {
		count := 0
		for change := range changes {
			t.Logf("backend2, change %v", count)
			err := backend2.Set(ctx, "background update", []SetRequest{change})
			if err != nil {
				t.Errorf("backend2, update %v: %v", count, err)
			}
			count++
		}
	})

	// Make a set of changes to a set of files
	for i := 0; i < 10; i++ {
		changes <- SetRequest{
			Path: fmt.Sprintf("test-%d", i),
			Doc: &StorageObject{
				Kind: "test",
				Body: []byte(fmt.Sprintf(`"test-%d"`, i)),
			},
		}
	}

	close(changes)

	wg.Wait()

}

func logWriter(t *testing.T) io.Writer {
	return &writer{
		t: t,
	}
}

type writer struct {
	t *testing.T
}

func (w *writer) Write(p []byte) (n int, err error) {
	for _, line := range strings.Split(string(p), "\n") {
		if len(line) == 0 {
			continue
		}
		w.t.Log(line)
	}
	return len(p), nil
}
