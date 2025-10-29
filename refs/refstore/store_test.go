package refstore

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func DoTestStore(t *testing.T, store Store) {
	t.Run("set/get", func(t *testing.T) {
		testStoreSetGet(t, store)
	})
	t.Run("link", func(t *testing.T) {
		testStoreLink(t, store)
	})
	t.Run("link_with_child_path", func(t *testing.T) {
		testStoreLinkWithChildPath(t, store)
	})
	t.Run("fragments", func(t *testing.T) {
		testStoreFragments(t, store)
	})
	t.Run("transaction", func(t *testing.T) {
		testStoreTransactions(t, store)
	})
}

func testStoreSetGet(t *testing.T, store Store) {
	ctx := context.Background()

	refFromInt := func(i int) string {
		return fmt.Sprintf("github.com/example/repo.git/path/to/package/@/custom/value%d", i)
	}

	var total int = 5

	var refSet = make(map[string]struct{})
	for i := 0; i < total; i++ {
		err := store.Set(ctx, refFromInt(i), fmt.Sprintf("value%d", i))
		if err != nil {
			t.Errorf("failed to set key%d: %v", i, err)
		}
		refSet[refFromInt(i)] = struct{}{}
	}

	for i := 0; i < total; i++ {
		var got string
		r := refFromInt(i)
		err := store.Get(ctx, r, &got)
		if err != nil {
			t.Errorf("failed to get key%d (%v): %v", i, r, err)
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
	for _, match := range matches {
		if !strings.HasPrefix(match, "github.com/example/repo.git/path/to/package/@/") {
			t.Errorf("ref was not a match: got %q, want prefix %q", match, "github.com/example/repo.git/path/to/package/@/")
		}
		if _, exists := refSet[match]; !exists {
			t.Errorf("ref was not in ref set: got %q", match)
		}
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
		t.Errorf("link should have been removed when overwritten: got %v, want %v", links, []string{})
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
		t.Errorf("overwritten link should be unaffected: got %v, want %v", links, []string{})
	}
}

func testStoreLinkWithChildPath(t *testing.T, store Store) {
	ctx := context.Background()

	// Create a link from @ to @r1
	linkSource := "minimal/repo/-/approvals.ocu.star/@v1"
	linkTarget := "minimal/repo/-/approvals.ocu.star/@r1"

	preExistingChildRef := "minimal/repo/-/approvals.ocu.star/@v1/custom/preexisting"
	createTestRefs(t, store, preExistingChildRef, "preexisting_value")

	// Create the target ref (links require the target to exist)
	// and the actual ref that will be accessed through the link
	createTestRefs(t, store, linkTarget, "r1_value")

	// Create the link
	if err := store.Link(ctx, linkSource, linkTarget); err != nil {
		t.Fatalf("failed to create link: %v", err)
	}

	// Get the value of the linked source
	checkRef(t, store, linkSource, "r1_value", "linked source value")

	// Resolve a child link
	linkedRef := "minimal/repo/-/approvals.ocu.star/@v1/custom/approval"
	resolved, err := store.ResolveLink(ctx, linkedRef)
	if resolved != "minimal/repo/-/approvals.ocu.star/@r1/custom/approval" {
		t.Fatalf("failed to resolve link: got %v %v", resolved, err)
	}

	// Set a child ref
	createTestRefs(t, store, linkedRef, "child_value")

	// Get the value of the linked ref
	checkRef(t, store, linkedRef, "child_value", "linked ref value")

	// Get the value of the linked ref at the resolved path
	checkRef(t, store, resolved, "child_value", "resolved ref value")

	// Check the preexisting child ref wasn't clobbered
	checkRef(t, store, preExistingChildRef, "preexisting_value", "preexisting child ref should not have been clobbered")
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

func testStoreTransactions(t *testing.T, store Store) {
	setRef(t, store, "tns/a", "b")
	setRef(t, store, "tns/todelete", "x")
	setRef(t, store, "before/should/not/match", "x")

	err := store.StartTransaction(t.Context(), "transaction1")
	if err != nil {
		t.Fatal(err)
	}

	checkRef(t, store, "tns/a", "b", "should see original value after starting transaction")

	setRef(t, store, "tns/a", "c")
	setRef(t, store, "tns/d", "e")

	if err := store.Link(t.Context(), "tns/link", "tns/d"); err != nil {
		t.Fatal(err)
	}

	checkRef(t, store, "tns/a", "c", "should see modified value mid-transaction")
	checkRef(t, store, "tns/d", "e", "should see new value mid-transaction")
	checkRef(t, store, "tns/link", "e", "link should be valid mid-transaction")

	deleteRef(t, store, "tns/todelete")

	checkRefNotExist(t, store, "tns/todelete", "deleted ref should not appear mid-transaction")

	// Ephemeral ref, only exists in the transaction
	setRef(t, store, "tns/ephemeral", "x")
	deleteRef(t, store, "tns/ephemeral")
	checkRefNotExist(t, store, "tns/ephemeral", "ephemeral ref should not exist after delete")

	setRef(t, store, "after/should/not/match", "x")

	matches, err := store.Match(t.Context(), "tns/**")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"tns/a", "tns/d", "tns/link"}
	if !cmp.Equal(matches, want) {
		t.Errorf("matches did not reflect transaction:\n%s", cmp.Diff(want, matches))
	}

	if err := store.CommitTransaction(t.Context()); err != nil {
		t.Fatal(err)
	}

	checkRef(t, store, "tns/a", "c", "should see modified value after commit")
	checkRef(t, store, "tns/d", "e", "should see new value after commit")
	checkRefNotExist(t, store, "tns/todelete", "deleted ref should not appear after commit")
	checkRef(t, store, "tns/link", "e", "link should be valid after commit")

	matches, err = store.Match(t.Context(), "tns/**")
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(matches, want) {
		t.Errorf("matches did not reflect committed transaction:\n%s", cmp.Diff(want, matches))
	}
}

func setRef(t *testing.T, store Store, ref string, value any) {
	t.Helper()
	if err := store.Set(t.Context(), ref, value); err != nil {
		t.Fatal(err)
	}
}

func deleteRef(t *testing.T, store Store, ref string) {
	t.Helper()
	if err := store.Delete(t.Context(), ref); err != nil {
		t.Fatal(err)
	}
}

func checkRef(t *testing.T, store Store, ref string, expected any, message string) {
	t.Helper()
	var v any
	if err := store.Get(t.Context(), ref, &v); err != nil {
		t.Fatalf("%s: failed to get key: %v", message, err)
	}
	if !cmp.Equal(v, expected) {
		t.Errorf("%s: got %v, want %v", message, v, expected)
	}
}

func checkRefNotExist(t *testing.T, store Store, ref string, message string) {
	t.Helper()
	var v any
	if err := store.Get(t.Context(), ref, &v); err != ErrRefNotFound {
		t.Fatalf("%v: %v", message, err)
	}
}
