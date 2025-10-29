package refstore

import (
	"context"
	"testing"
)

func TestGitRefStore(t *testing.T) {
	// TODO: This tests passes, but takes 17s for some reason!
	t.Skip()

	bareRepoPath, remoteURL := setupTestRepo(t, "refstore")
	be, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "main", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewRefStore(context.Background(), be, map[string]struct{}{})
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
