package refstore

import (
	"context"
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
