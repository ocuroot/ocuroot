package refstore

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRefStore(t *testing.T) {
	store, err := NewRefStore(
		context.Background(),
		NewInMemoryBackend(),
		map[string]struct{}{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()

	DoTestStore(t, store)
}

// TestRefStorePaths verifies that the internal structure for
// storing refs is as expected.
// This ensures backwards compatibility with existing backends.
func TestRefStorePaths(t *testing.T) {
	ctx := context.Background()

	backend := NewInMemoryBackend()
	store, err := NewRefStore(
		ctx,
		backend,
		map[string]struct{}{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()

	err = store.Set(ctx, "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	err = store.Set(ctx, "d/e", "f")
	if err != nil {
		t.Fatal(err)
	}

	out, err := backend.Get(ctx, []string{
		"refs/a/@object.json",
		"refs/d/e/@object.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Errorf("expected 2 results, got %d", len(out))
	}
	if out[0].Doc == nil || out[1].Doc == nil {
		t.Errorf("expected non-nil docs, got %+v", out)
	}
	if string(out[0].Doc.Body) != `"b"` || string(out[1].Doc.Body) != `"f"` {
		t.Errorf("expected b and f, got %+v", out)
	}
}

// TestNewRefStoreWithTags verifies that tags are properly set when creating a new refstore
func TestNewRefStoreWithTags(t *testing.T) {
	ctx := context.Background()

	tags := map[string]struct{}{
		"source": {},
		"test":   {},
	}

	backend := NewInMemoryBackend()
	store, err := NewRefStore(ctx, backend, tags)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Check that tags are set in the store info
	info := store.Info()

	if info.Version != stateVersion {
		t.Errorf("expected version %d, got %d", stateVersion, info.Version)
	}

	if len(info.Tags) != len(tags) {
		t.Errorf("expected %d tags, got %d", len(tags), len(info.Tags))
	}

	for tag := range tags {
		if _, ok := info.Tags[tag]; !ok {
			t.Errorf("expected tag %q to be present", tag)
		}
	}
}

// TestNewRefStoreExistingBackend verifies that opening an existing backend preserves its data
func TestNewRefStoreExistingBackend(t *testing.T) {
	ctx := context.Background()

	tags := map[string]struct{}{
		"source": {},
	}

	backend := NewFsBackend(t.TempDir())

	// Create initial store and set some data
	store1, err := NewRefStore(ctx, backend, tags)
	if err != nil {
		t.Fatal(err)
	}

	if err := store1.Set(ctx, "test/ref", "value1"); err != nil {
		t.Fatal(err)
	}

	if err := store1.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen the same backend with the same tags
	store2, err := NewRefStore(ctx, backend, tags)
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()

	// Verify the data is still there
	var got string
	if err := store2.Get(ctx, "test/ref", &got); err != nil {
		t.Fatal(err)
	}
	if got != "value1" {
		t.Errorf("expected value1, got %q", got)
	}

	// Verify tags are preserved
	info := store2.Info()
	if _, ok := info.Tags["source"]; !ok {
		t.Errorf("expected source tag to be preserved")
	}
}

// TestNewRefStoreVersionUpgrade tests upgrading from version 2 to version 3
func TestNewRefStoreVersionUpgrade(t *testing.T) {
	ctx := context.Background()

	backend := NewInMemoryBackend()

	// Manually create a version 2 store info
	v2Info := &StoreInfo{
		Version: 2,
		Tags:    map[string]struct{}{}, // Version 2 had tags but they might be empty
	}
	v2InfoBytes, err := json.Marshal(v2Info)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.SetBytes(ctx, storeInfoFile, v2InfoBytes); err != nil {
		t.Fatal(err)
	}

	// Now open with NewRefStore which should upgrade to version 3
	newTags := map[string]struct{}{
		"upgraded": {},
		"test":     {},
	}
	store, err := NewRefStore(ctx, backend, newTags)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Verify it was upgraded to version 3
	info := store.Info()

	if info.Version != 3 {
		t.Errorf("expected version 3 after upgrade, got %d", info.Version)
	}

	// Verify new tags were set
	if len(info.Tags) != len(newTags) {
		t.Errorf("expected %d tags, got %d", len(newTags), len(info.Tags))
	}

	for tag := range newTags {
		if _, ok := info.Tags[tag]; !ok {
			t.Errorf("expected tag %q to be present after upgrade", tag)
		}
	}
}

// TestNewRefStoreIncompatibleVersion tests that incompatible versions are rejected
func TestNewRefStoreIncompatibleVersion(t *testing.T) {
	ctx := context.Background()

	backend := NewInMemoryBackend()

	// Manually create a store info with an incompatible version
	incompatibleInfo := &StoreInfo{
		Version: 999,
		Tags:    map[string]struct{}{},
	}
	incompatibleBytes, err := json.Marshal(incompatibleInfo)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.SetBytes(ctx, storeInfoFile, incompatibleBytes); err != nil {
		t.Fatal(err)
	}

	// Attempt to open should fail
	_, err = NewRefStore(ctx, backend, map[string]struct{}{})
	if err == nil {
		t.Fatal("expected error when opening incompatible version, got nil")
	}

	expectedMsg := "incompatible store version"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("expected error to contain %q, got %q", expectedMsg, err.Error())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
