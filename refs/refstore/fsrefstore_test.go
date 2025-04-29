package refstore

import (
	"os"
	"testing"
)

func TestFSRefStore(t *testing.T) {
	dir, err := os.MkdirTemp("", "ocuroot-release-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	store, err := NewFSRefStore(dir)
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
