package refstore

import (
	"os"
	"testing"
)

func TestFSRefStore(t *testing.T) {
	tempDir := "./testdata/fs_testdata"
	_ = os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}

	store, err := NewFSRefStore(tempDir, map[string]struct{}{})
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
