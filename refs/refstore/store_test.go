package refstore

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func DoTestStore(t *testing.T, store Store) {
	t.Run("set/get", func(t *testing.T) {
		testStoreSetGet(t, store)
	})
	t.Run("link", func(t *testing.T) {
		testStoreLink(t, store)
	})
	t.Run("dependencies", func(t *testing.T) {
		testStoreDependencies(t, store)
	})
	t.Run("fragments", func(t *testing.T) {
		testStoreFragments(t, store)
	})
}

func testStoreSetGet(t *testing.T, store Store) {
	ctx := context.Background()

	refFromInt := func(i int) string {
		return fmt.Sprintf("github.com/example/repo.git/path/to/package/@/custom/value%d", i)
	}

	var total int = 5

	for i := 0; i < total; i++ {
		err := store.Set(ctx, refFromInt(i), fmt.Sprintf("value%d", i))
		if err != nil {
			t.Errorf("failed to set key%d: %v", i, err)
		}
	}

	for i := 0; i < total; i++ {
		var got string
		err := store.Get(ctx, refFromInt(i), &got)
		if err != nil {
			t.Errorf("failed to get key%d: %v", i, err)
		}
		if got != fmt.Sprintf("value%d", i) {
			t.Errorf("unexpected value for key%d: got %q, want %q", i, got, fmt.Sprintf("value%d", i))
		}
	}

	// Check for matches
	matches, err := store.Match(ctx, "github.com/example/repo.git/path/to/package/@/**")
	if err != nil {
		t.Errorf("failed to match keys: %v", err)
	}
	if len(matches) != total {
		t.Errorf("unexpected number of matches: got %d, want %d", len(matches), total)
	}

	for i := 0; i < total; i++ {
		err := store.Delete(ctx, refFromInt(i))
		if err != nil {
			t.Errorf("failed to delete key%d: %v", i, err)
		}
	}

	for i := 0; i < total; i++ {
		var got string
		err := store.Get(ctx, refFromInt(i), &got)
		if err != ErrRefNotFound {
			t.Errorf("expected ErrNotFound for key%d, got %v", i, err)
		}
		if got != "" {
			t.Errorf("unexpected value for key%d: got %q, want %q", i, got, "")
		}
	}

	matches, err = store.Match(ctx, "github.com/example/repo.git/path/to/package/@/**")
	if err != nil {
		t.Errorf("failed to match keys: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("unexpected number of matches: got %d, want %d", len(matches), 0)
	}
}

func createTestRefs(t *testing.T, store Store, refsAndValues ...any) {
	ctx := context.Background()
	for i := 0; i < len(refsAndValues); i += 2 {
		if err := store.Set(ctx, refsAndValues[i].(string), refsAndValues[i+1]); err != nil {
			t.Errorf("failed to set key%d: %v", i, err)
		}
		t.Cleanup(func() {
			if err := store.Delete(ctx, refsAndValues[i].(string)); err != nil {
				t.Errorf("failed to delete key%d: %v", i, err)
			}
		})
	}
}

func testStoreLink(t *testing.T, store Store) {
	ctx := context.Background()

	ref := "github.com/example/repo.git/1/package/first/@abc123/deploy/staging"
	ref2 := "github.com/example/repo.git/1/package/first/@abc123/deploy/staging/child"
	link := "github.com/example/repo.git/1/package/first/@/deploy/staging"

	ref3 := "github.com/example/repo.git/1/package/first/@def456/deploy/staging"
	link2 := "github.com/example/repo.git/1/package/first/@/deploy/staging/def456"

	createTestRefs(t, store,
		ref, "value1",
		ref2, "value2",
		ref3, "value3",
	)

	if err := store.Link(ctx, link, ref); err != nil {
		t.Errorf("failed to link: %v", err)
	}
	if err := store.Link(ctx, link2, ref3); err != nil {
		t.Errorf("failed to link: %v", err)
	}

	var got string
	if err := store.Get(ctx, link, &got); err != nil {
		t.Errorf("failed to get link: %v", err)
	}
	if got != "value1" {
		t.Errorf("unexpected value for link: got %q, want %q", got, "value1")
	}

	var got2 string
	if err := store.Get(ctx, filepath.Join(link, "child"), &got2); err != nil {
		t.Errorf("failed to get link: %v", err)
	}
	if got2 != "value2" {
		t.Errorf("unexpected value for link: got %q, want %q", got2, "value2")
	}

	var got3 string
	if err := store.Get(ctx, link2, &got3); err != nil {
		t.Errorf("failed to get link: %v", err)
	}
	if got3 != "value3" {
		t.Errorf("unexpected value for link: got %q, want %q", got3, "value3")
	}

	// Confirm GetLinks works
	links, err := store.GetLinks(ctx, ref)
	if err != nil {
		t.Errorf("failed to get links: %v", err)
	}
	if len(links) != 1 || links[0] != link {
		t.Errorf("unexpected links: got %v, want %v", links, []string{link})
	}

	// Confirm that changing the value retains links
	if err := store.Set(ctx, ref, "value4"); err != nil {
		t.Errorf("failed to set key: %v", err)
	}

	links, err = store.GetLinks(ctx, ref)
	if err != nil {
		t.Errorf("failed to get links: %v", err)
	}
	if len(links) != 1 || links[0] != link {
		t.Errorf("unexpected links after set: got %v, want %v", links, []string{link})
	}

	if err := store.Link(ctx, link, ref2); err != nil {
		t.Errorf("failed to link: %v", err)
	}

	var got4 string
	if err := store.Get(ctx, link, &got4); err != nil {
		t.Errorf("failed to get link: %v", err)
	}
	if got4 != "value2" {
		t.Errorf("unexpected value for link: got %q, want %q", got4, "value2")
	}

	links, err = store.GetLinks(ctx, ref)
	if err != nil {
		t.Errorf("failed to get links: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("unexpected links: got %v, want %v", links, []string{})
	}

	links, err = store.GetLinks(ctx, ref2)
	if err != nil {
		t.Errorf("failed to get links: %v", err)
	}
	if len(links) != 1 || links[0] != link {
		t.Errorf("unexpected links: got %v, want %v", links, []string{link})
	}

	if err := store.Unlink(ctx, link); err != nil {
		t.Errorf("failed to unlink: %v", err)
	}

	links, err = store.GetLinks(ctx, ref)
	if err != nil {
		t.Errorf("failed to get links: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("unexpected links: got %v, want %v", links, []string{})
	}
}

func testStoreDependencies(t *testing.T, store Store) {
	ctx := context.Background()

	ref1 := "github.com/example/repo.git/1/package/first/@/deploy/staging"
	ref2 := "github.com/example/repo.git/second/package/@/deploy/production"
	ref3 := "github.com/example/alternative.git/third/@/deploy/staging"

	createTestRefs(t, store,
		ref1, "value1",
		ref2, "value2",
		ref3, "value3",
	)

	err := store.AddDependency(ctx, ref1, ref2)
	if err != nil {
		t.Errorf("failed to add dependency: %v", err)
	}
	t.Cleanup(func() {
		if err := store.RemoveDependency(ctx, ref1, ref2); err != nil {
			t.Errorf("failed to remove dependency: %v", err)
		}
	})
	err = store.AddDependency(ctx, ref1, ref3)
	if err != nil {
		t.Errorf("failed to add dependency: %v", err)
	}
	t.Cleanup(func() {
		if err := store.RemoveDependency(ctx, ref1, ref3); err != nil {
			t.Errorf("failed to remove dependency: %v", err)
		}
	})

	// ref1 should have the above dependencies
	gotDeps, err := store.GetDependencies(ctx, ref1)
	if err != nil {
		t.Errorf("failed to get dependencies: %v", err)
	}
	if len(gotDeps) != 2 {
		t.Errorf("unexpected number of dependencies: got %d, want %d", len(gotDeps), 2)
	}
	if len(gotDeps) >= 2 && (gotDeps[0] != ref3 || gotDeps[1] != ref2) {
		t.Errorf("unexpected dependencies: got %v, want %v", gotDeps, []string{ref2, ref3})
	}

	// ref2 should have no dependencies
	gotDeps, err = store.GetDependencies(ctx, ref2)
	if err != nil {
		t.Errorf("failed to get dependencies: %v", err)
	}
	if len(gotDeps) != 0 {
		t.Log(gotDeps)
		t.Errorf("unexpected number of dependencies: got %d, want %d", len(gotDeps), 0)
	}

	// ref3 should have ref1 as a dependant
	gotDeps, err = store.GetDependants(ctx, ref3)
	if err != nil {
		t.Errorf("failed to get dependants: %v", err)
	}
	if len(gotDeps) != 1 {
		t.Errorf("unexpected number of dependants: got %d, want %d", len(gotDeps), 1)
	}
	if len(gotDeps) > 0 && gotDeps[0] != ref1 {
		t.Errorf("unexpected dependant: got %v, want %v", gotDeps[0], ref1)
	}
}

func testStoreFragments(t *testing.T, store Store) {
	ctx := context.Background()

	createTestRefs(t, store,
		"repo.git/package/@/custom/value", struct {
			Field1 string            `json:"field1"`
			Field2 string            `json:"field2"`
			Map    map[string]string `json:"map"`
		}{
			Field1: "value",
			Field2: "value2",
			Map: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
	)

	var got string
	if err := store.Get(ctx, "repo.git/package/@/custom/value#field1", &got); err != nil {
		t.Errorf("failed to get key: %v", err)
	}
	if got != "value" {
		t.Errorf("unexpected value for key: got %q, want %q", got, "value")
	}

	if err := store.Get(ctx, "repo.git/package/@/custom/value#map/key1", &got); err != nil {
		t.Errorf("failed to get key: %v", err)
	}
	if got != "value1" {
		t.Errorf("unexpected value for key: got %q, want %q", got, "value1")
	}
}
