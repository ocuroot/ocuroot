package refstore

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
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

	if err := backend.Set(context.Background(), nil, "test", []SetRequest{
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
	err = backend.Set(context.Background(), nil, "test", []SetRequest{
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
}

func doTestBackendMatch(t *testing.T, getBackend func() DocumentBackend) {
	backend := getBackend()
	var results []string
	var err error
	var reqs []MatchRequest
	results, err = backend.Match(context.Background(), []MatchRequest{
		{
			Prefix: "subpath",
			Glob:   "**",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}

	if err := backend.Set(context.Background(), nil, "test", []SetRequest{
		{
			Path: "subpath/one/@object.json",
			Doc: &StorageObject{
				Kind: "test",
				Body: []byte(`"test"`),
			},
		},
		{
			Path: "subpath/two/@object.json",
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
	}); err != nil {
		t.Fatal(err)
	}

	reqs = []MatchRequest{
		{
			Prefix: "subpath",
			Glob:   "**",
		},
	}
	results, err = backend.Match(context.Background(), reqs)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("%+v: expected 2 results, got %+v", reqs, results)
	} else {
		sort.Strings(results)
		if results[0] != "subpath/one/@object.json" {
			t.Errorf("%+v: expected 'subpath/one/@object.json', got '%s'", reqs, results[0])
		}
		if results[1] != "subpath/two/@object.json" {
			t.Errorf("%+v: expected 'subpath/two/@object.json', got '%s'", reqs, results[1])
		}
	}

	reqs = []MatchRequest{
		{
			Prefix:   "",
			Suffixes: []string{"/@object.json"},
			Glob:     "**/one",
		},
	}
	results, err = backend.Match(context.Background(), reqs)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("%+v: expected 1 result, got %+v", reqs, results)
	} else if results[0] != "subpath/one/@object.json" {
		t.Errorf("%+v: expected 'subpath/one/@object.json', got '%s'", reqs, results[0])
	}
}
